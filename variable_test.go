package hc

import (
	"testing"
)

func TestNewVariableManager(t *testing.T) {
	vm := NewVariableManager(make(map[string]string))
	if vm == nil {
		t.Fatal("NewVariableManager returned nil")
	}
	if vm.variables == nil {
		t.Fatal("variables map is nil")
	}
}

func TestVariableManager_SetAndGet(t *testing.T) {
	vm := NewVariableManager(make(map[string]string))

	// Set a string value
	vm.Set("name", "John", make(map[string]any))
	if vm.Get("name") != "John" {
		t.Errorf("Expected 'John', got %v", vm.Get("name"))
	}

	// Set an int value
	vm.Set("age", "30", make(map[string]any))
	if vm.Get("age") != "30" {
		t.Errorf("Expected 30, got %v", vm.Get("age"))
	}

	// Get non-existent key
	if vm.Get("nonexistent") != "" {
		t.Errorf("Expected empty string for non-existent key, got %v", vm.Get("nonexistent"))
	}
}

func TestVariableManager_ReplaceVariables(t *testing.T) {
	vm := NewVariableManager(make(map[string]string))
	vm.Set("userId", "123", make(map[string]any))
	vm.Set("token", "abc123", make(map[string]any))

	tests := []struct {
		input    string
		expected string
	}{
		{"Hello {{userId}}", "Hello 123"},
		{"Token: {{token}}", "Token: abc123"},
		{"No variables here", "No variables here"},
		{"{{userId}} and {{token}}", "123 and abc123"},
		{"{{nonexistent}}", "{{nonexistent}}"}, // Non-existent variable should remain unchanged
		{"{{userId", "{{userId"},               // Incomplete template
		{"userId}}", "userId}}"},               // Incomplete template
	}

	for _, test := range tests {
		result := vm.ReplaceVariables(test.input)
		if result != test.expected {
			t.Errorf("ReplaceVariables(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}
