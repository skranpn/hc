package hc

import (
	"regexp"
	"testing"
)

func TestVariableManager_SetAndGet(t *testing.T) {
	vm := NewVariableManager(make(map[string]string))

	// Set a string value
	vm.Set("name", "John")
	if vm.Get("name") != "John" {
		t.Errorf("Expected 'John', got %v", vm.Get("name"))
	}

	// Set an int value
	vm.Set("age", "30")
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
	vm.Set("userId", "123")
	vm.Set("token", "abc123")
	vm.SetJSONPaths(map[string]any{
		"test.response.status": 200,
	})

	tests := []struct {
		input    string
		expected string
	}{
		{"Hello {{userId}}", "Hello 123"},
		{"No variables here", "No variables here"},
		{"{{userId}} and {{token}}", "123 and abc123"},
		{"{{nonexistent}}", "{{nonexistent}}"}, // Non-existent variable should remain unchanged
		{"{{userId", "{{userId"},               // Incomplete template
		{"userId}}", "userId}}"},               // Incomplete template
		// jsonpath
		{"{{test.response.status}}", "200"},
		{"{{test.response.body.id}}", "{{test.response.body.id}}"}, // Non-existent jsonpath variable
	}

	for _, test := range tests {
		result := vm.ReplaceVariables(test.input)
		if result != test.expected {
			t.Errorf("ReplaceVariables(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestVariableManager_ReplaceSystemVariables(t *testing.T) {
	vm := NewVariableManager(make(map[string]string))

	tests := []struct {
		input   string
		pattern string
	}{
		{"{{$guid}}", `^[a-z0-9]{8}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{12}$`},
		{"{{$randomInt}}", `^[0-9]+$`},
		{"{{$randomInt 7}}", `^[0-9]{1}$`},
		{"{{$randomInt 0 10}}", `^[0-9]{1,2}$`},
		{"{{$randomInt -10 -5}}", `^-[0-9]{1,2}$`},
		{"{{$timestamp}}", `^[0-9]{10}$`},
		{"{{$timestamp 1 m}}", `^[0-9]{10}$`},
	}

	for _, test := range tests {
		result := vm.ReplaceVariables(test.input)
		expected := regexp.MustCompile(test.pattern)
		if !expected.MatchString(result) {
			t.Errorf("ReplaceSystemVariables(%q) = %q, expected %q", test.input, result, test.pattern)
		}
	}
}
