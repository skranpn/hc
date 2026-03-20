package hc

import (
	"context"
	"testing"

	"github.com/skranpn/hc/metadata"
)

func TestHandleMetadataVariable(t *testing.T) {
	testdata := []struct {
		name     string
		env      map[string]string
		req      HttpRequest
		res      HttpResponse
		variable string
		expect   string
	}{{
		"==",
		nil,
		HttpRequest{
			Name:     "TEST",
			Metadata: metadata.Metadata{GetMetadataVariable(t, "id = {{TEST.response.body.keys[?@.value==10].id}}")},
		},
		HttpResponse{Body: []byte(`{"keys":[{"id":1,"value":0},{"id":2,"value":10},{"id":3,"value":20}]}`)},
		"{{id}}",
		"2",
	}, {
		"match",
		nil,
		HttpRequest{Name: "TEST", Metadata: metadata.Metadata{GetMetadataVariable(t, "id = {{TEST.response.body.keys[?match(@.value,'some.*')].id}}")}},
		HttpResponse{Body: []byte(`{"keys":[{"id":1,"value":"something-long-text"},{"id":2,"value":"for-example-uuid-here"}]}`)},
		"{{id}}",
		"1",
	}, {
		"nested",
		map[string]string{"env": "dev"},
		HttpRequest{Name: "TEST", Metadata: metadata.Metadata{GetMetadataVariable(t, "id = {{TEST.response.body.keys[?match(@.value,'{{env}}.*')].id}}")}},
		HttpResponse{Body: []byte(`{"keys":[{"id":1,"value":"hello"},{"id":2,"value":"test-value"},{"id":3,"value":"dev-value"}]}`)},
		"{{id}}",
		"3",
	}}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			vm := NewVariableManager(tt.env)
			ch := make(chan *Report)
			go func() {
				for {
					<-ch
				}
			}()
			runner := NewRunner(nil, vm, nil, ch)
			err := runner.handleMetadata(context.TODO(), &tt.req, &tt.res)
			if err != nil {
				t.Fatalf("%s: failed to handleMetadata: %v", tt.name, err)
			}

			if runner.vm.ReplaceVariables(tt.variable) != tt.expect {
				t.Fatalf("%s: invalid variable, want: %s, got: %q", tt.name, tt.expect, runner.vm.Get(tt.variable))
			}
		})
	}
}

func GetMetadataVariable(t *testing.T, expr string) *metadata.Variable {
	t.Helper()

	v, _ := metadata.NewVariable(expr)
	return v
}
