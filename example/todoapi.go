package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Todo represents a todo item
type Todo struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

// TodoServer manages todos in memory
type TodoServer struct {
	todos  map[int]*Todo
	mu     sync.RWMutex
	nextID int
}

// NewTodoServer creates a new todo server
func NewTodoServer() *TodoServer {
	return &TodoServer{
		todos:  make(map[int]*Todo),
		nextID: 1,
	}
}

// GetTodos returns all todos
func (s *TodoServer) GetTodos(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	todos := make([]*Todo, 0, len(s.todos))
	for _, todo := range s.todos {
		todos = append(todos, todo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todos)
}

// CreateTodo creates a new todo
func (s *TodoServer) CreateTodo(w http.ResponseWriter, r *http.Request) {
	var todo Todo
	if err := json.NewDecoder(r.Body).Decode(&todo); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	todo.ID = s.nextID
	s.nextID++
	s.todos[todo.ID] = &todo

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(todo)
}

// GetTodo returns a specific todo
func (s *TodoServer) GetTodo(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path)
	if id == -1 {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	todo, ok := s.todos[id]
	if !ok {
		http.Error(w, "Todo not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todo)
}

// UpdateTodo updates a todo
func (s *TodoServer) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path)
	if id == -1 {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var todoUpdate Todo
	if err := json.NewDecoder(r.Body).Decode(&todoUpdate); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	todo, ok := s.todos[id]
	if !ok {
		http.Error(w, "Todo not found", http.StatusNotFound)
		return
	}

	if todoUpdate.Title != "" {
		todo.Title = todoUpdate.Title
	}
	todo.Completed = todoUpdate.Completed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todo)
}

// DeleteTodo deletes a todo
func (s *TodoServer) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path)
	if id == -1 {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.todos[id]; !ok {
		http.Error(w, "Todo not found", http.StatusNotFound)
		return
	}

	delete(s.todos, id)
	w.WriteHeader(http.StatusNoContent)
}

// extractID extracts the ID from the URL path
func extractID(path string) int {
	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return -1
	}
	id, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return -1
	}
	return id
}

func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "can't read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		fmt.Printf("REQ: %s %s, body: %s\n", r.Method, r.URL, string(bodyBytes))
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		next.ServeHTTP(w, r)
	})
}

// Start starts the todo API server
func Start(addr string) error {
	server := NewTodoServer()

	mux := http.NewServeMux()
	mux.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			server.GetTodos(w, r)
		case http.MethodPost:
			server.CreateTodo(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			server.GetTodo(w, r)
		case http.MethodPut:
			server.UpdateTodo(w, r)
		case http.MethodDelete:
			server.DeleteTodo(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return http.ListenAndServe(addr, RequestLoggerMiddleware(mux))
}

func main() {
	if err := Start("localhost:3000"); err != nil {
		panic(err)
	}
}
