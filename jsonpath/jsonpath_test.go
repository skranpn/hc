package jsonpath

import "testing"

func TestSerialize(t *testing.T) {
	testdata := []struct {
		name string
		data string
		want string
	}{
		{"status code", `GetStatusCode.response.status`, "$.response.status"},
		{"header", `GetStatusCode.response.headers.Content-Type`, "$.response.headers['Content-Type']"},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			got := serialize(tt.data)

			if got != tt.want {
				t.Fatalf("Expected %s, got %s", tt.want, got)
			}
		})
	}
}
