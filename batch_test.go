package hc

import (
	"testing"

	"github.com/skranpn/hc/metadata"
)

func TestParallelStages(t *testing.T) {
	testdata := []struct {
		name     string
		requests []HttpRequest
		want     int
	}{
		{"single", []HttpRequest{{Name: "GET", Method: "GET", URL: "example.com"}}, 1},
		// 1: GET, GET, GET
		{"nodeps", []HttpRequest{
			{Name: "GET", Method: "GET", URL: "example.com"},
			{Name: "GET", Method: "GET", URL: "example.com"},
			{Name: "GET", Method: "GET", URL: "example.com"},
		}, 1},
		// 1: CREATE
		// 2: GET, DELETE
		{"variable deps", []HttpRequest{
			{Name: "CREATE", Method: "POST", URL: "example.com/todos",
				Metadata: metadata.Metadata{&metadata.Variable{Name: "id", Value: "{{CREATE.response.body.id}}"}},
			},
			{Name: "GET", Method: "GET", URL: "example.com/todos/{{id}}"},
			{Name: "DELETE", Method: "DELETE", URL: "example.com/todos/{{id}}"},
		}, 2},
		// 1: CREATE
		// 2: GET, DELETE
		{"jsonpath deps", []HttpRequest{
			{Name: "CREATE", Method: "POST", URL: "example.com/todos"},
			{Name: "GET", Method: "GET", URL: "example.com/todos/{{CREATE.response.body.id}}"},
			{Name: "DELETE", Method: "DELETE", URL: "example.com/todos/{{CREATE.response.body.id}}"},
		}, 2},
		// 1: CREATE
		// 2: GET, DELETE
		{"mixed deps", []HttpRequest{
			{Name: "CREATE", Method: "POST", URL: "example.com/todos",
				Metadata: metadata.Metadata{&metadata.Variable{Name: "id", Value: "{{CREATE.response.body.id}}"}},
			},
			{Name: "GET", Method: "GET", URL: "example.com/todos/{{CREATE.response.body.id}}"},
			{Name: "DELETE", Method: "DELETE", URL: "example.com/todos/{{id}}"},
		}, 2},
		// 1: health,CREATE
		// 2: GET, DELETE
		{"all", []HttpRequest{
			{Name: "healthcheck", Method: "GET", URL: "example.com/health"},
			{Name: "CREATE", Method: "POST", URL: "example.com/todos",
				Metadata: metadata.Metadata{&metadata.Variable{Name: "id", Value: "{{CREATE.response.body.id}}"}},
			},
			{Name: "GET", Method: "GET", URL: "example.com/todos/{{CREATE.response.body.id}}"},
			{Name: "DELETE", Method: "DELETE", URL: "example.com/todos/{{id}}"},
		}, 2},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {

			resolver := NewDependencyResolver(tt.requests)
			stages := resolver.BuildExecutionPlan()

			if len(stages) != tt.want {
				t.Errorf("Expected %d stages, got: %d", tt.want, len(stages))
			}
		})
	}
}
