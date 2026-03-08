package hc_test

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/skranpn/hc"

	"github.com/google/go-cmp/cmp"
)

func TestParseRequestCount(t *testing.T) {
	testdata := []struct {
		name string
		data string
		want int
	}{
		{"get_once", `GET http://example.com`, 1},
		{"get_twice", `GET http://example.com
		               ###
		               GET http://example.com`, 2},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			parser := hc.NewParser()

			requests, err := parser.Parse(bytes.NewReader([]byte(tt.data)))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if len(requests) != tt.want {
				t.Errorf("Expected %d requests, got %d", tt.want, len(requests))
			}
		})
	}
}

func TestParseRequestLine(t *testing.T) {
	testdata := []struct {
		name    string
		data    string
		methods []string
		urls    []string
	}{
		{"get", `GET http://example.com`, []string{"GET"}, []string{"http://example.com"}},
		{"post", `POST http://example.com`, []string{"POST"}, []string{"http://example.com"}},
		{"get,post", `GET http://example.com/get
		              ###
		              POST http://example.com/post`,
			[]string{"GET", "POST"},
			[]string{"http://example.com/get", "http://example.com/post"}},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			parser := hc.NewParser()

			requests, err := parser.Parse(bytes.NewReader([]byte(tt.data)))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			methods := make([]string, 0, len(requests))
			urls := make([]string, 0, len(requests))
			for _, request := range requests {
				methods = append(methods, request.Method)
				urls = append(urls, request.URL)
			}

			if diff := cmp.Diff(tt.methods, methods); diff != "" {
				t.Fatalf("%s: methods mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.urls, urls); diff != "" {
				t.Fatalf("%s: urls mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

func TestParseHeaders(t *testing.T) {
	testdata := []struct {
		name string
		data string
		want []map[string]string
	}{
		{"single", `GET http://example.com
			        Accept: application/json`,
			[]map[string]string{{"Accept": "application/json"}},
		},
		{"double", `GET http://example.com
			        Host: http://example.com
			        Authorization: Bearer token`,
			[]map[string]string{{
				"Host":          "http://example.com",
				"Authorization": "Bearer token",
			}},
		},
		{"two", `GET http://example.com/get
			     X-My-Header: hello

			     ###

			     POST http://example.com/post
				 Content-Type: application/json
				 Content-Length: 0`,
			[]map[string]string{{
				"X-My-Header": "hello",
			}, {
				"Content-Type":   "application/json",
				"Content-Length": "0",
			}},
		},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			parser := hc.NewParser()

			requests, err := parser.Parse(bytes.NewReader([]byte(tt.data)))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			headers := make([]map[string]string, 0, len(requests))
			for _, request := range requests {
				headers = append(headers, request.Headers)
			}

			if diff := cmp.Diff(tt.want, headers); diff != "" {
				t.Fatalf("%s: methods mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}

}

func TestParseBody(t *testing.T) {
	testdata := []struct {
		name string
		data string
		want []string
	}{
		{"get", `GET http://example.com
			        Accept: application/json`, []string{""}},
		{"json", `POST http://example.com
			      Content-Type: application/json
					
{"message":"hello"}`,
			[]string{`{"message":"hello"}`},
		},
		{"two", `POST http://example.com/post
			     X-My-Header: hello

{"id":0, "iid":0}

			     ###

			     POST http://example.com/post
				 Content-Type: application/json
			
{
  "id": 1,
  "iid": 2
}`, []string{`{"id":0, "iid":0}`, `{
  "id": 1,
  "iid": 2
}`},
		},
		{"whitespace", `POST http://example.com
		
 `, []string{" "}},
	}

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			parser := hc.NewParser()

			requests, err := parser.Parse(bytes.NewReader([]byte(tt.data)))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			bodies := make([]string, 0, len(requests))
			for _, request := range requests {
				bodies = append(bodies, request.Body)
			}

			if diff := cmp.Diff(tt.want, bodies); diff != "" {
				t.Fatalf("%s: methods mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}

}

func TestParseName(t *testing.T) {
	name := "testrequest"
	testdata := `
	# @name ` + name + `
	GET http://example.com`

	parser := hc.NewParser()
	requests, err := parser.Parse(bytes.NewReader([]byte(testdata)))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(requests) == 0 {
		t.Fatal("no requests")
	}
	if requests[0].Name != name {
		t.Errorf("invalid name, want: %s, got: %s", name, requests[0].Name)
	}
}

func TestParseAssert(t *testing.T) {
	left := "GET.response.status"
	op := "=="
	right := "200"
	testdata := `
	# @name = GET
	GET http://example.com

	# @assert ` + left + ` ` + op + ` ` + right

	parser := hc.NewParser()
	requests, err := parser.Parse(bytes.NewReader([]byte(testdata)))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(requests[0].Metadata) == 0 {
		t.Fatal("No assert")
	}
	metadata, ok := requests[0].Metadata[0].(*hc.Assertion)
	if !ok {
		t.Fatal("metadata is not assert")
	}
	if metadata.LeftPath != left {
		t.Errorf("invalid left, want: %s, got: %s", left, metadata.LeftPath)
	}
	if metadata.Operator != op {
		t.Errorf("invalid operator, want: %s, got: %s", op, metadata.Operator)
	}
	if metadata.RightValue != right {
		t.Errorf("invalid right, want: %s, got: %s", right, metadata.RightValue)
	}

}

func TestParseUntil(t *testing.T) {
	left := "GET.response.status"
	op := "=="
	right := "200"
	maxStr := "10"
	intervalStr := "10"
	testdata := `
	# @name = GET
	GET http://example.com

	# @until ` + left + ` ` + op + ` ` + right + ` max ` + maxStr + ` interval ` + intervalStr

	parser := hc.NewParser()
	requests, err := parser.Parse(bytes.NewReader([]byte(testdata)))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(requests[0].Metadata) == 0 {
		t.Fatal("No assert")
	}
	metadata, ok := requests[0].Metadata[0].(*hc.Until)
	if !ok {
		t.Fatal("metadata is not until")
	}
	if metadata.Condition.LeftPath != left {
		t.Errorf("invalid left, want: %s, got: %s", left, metadata.Condition.LeftPath)
	}
	if metadata.Condition.Operator != op {
		t.Errorf("invalid operator, want: %s, got: %s", op, metadata.Condition.Operator)
	}
	if metadata.Condition.RightValue != right {
		t.Errorf("invalid right, want: %s, got: %s", right, metadata.Condition.RightValue)
	}
	max, _ := strconv.Atoi(maxStr)
	if metadata.MaxRetry != max {
		t.Errorf("invalid max, want: %d, got: %d", max, metadata.MaxRetry)
	}
	intervalInt, _ := strconv.Atoi(intervalStr)
	interval := time.Duration(intervalInt) * time.Second
	if metadata.Interval != interval {
		t.Errorf("invalid interval, want: %d, got: %d", interval, metadata.Interval)
	}
}

func TestParseVariable(t *testing.T) {
	key1 := "key1"
	value1 := "value1"
	key2 := "key2"
	value2 := "value2"
	testdata := `
	# @name = GET
	GET http://example.com

	# @` + key1 + ` = ` + value1 + `
	# @` + key2 + ` = ` + value2

	parser := hc.NewParser()
	requests, err := parser.Parse(bytes.NewReader([]byte(testdata)))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(requests[0].Metadata) != 2 {
		t.Fatalf("invalid variable count, want: 2, got: %d", len(requests[0].Metadata))
	}
	metadata, ok := requests[0].Metadata[0].(*hc.Variable)
	if !ok {
		t.Fatal("metadata[0] is not variable")
	}
	if metadata.Name != key1 {
		t.Errorf("invalid name, want: %s, got: %s", key1, metadata.Name)
	}
	if metadata.Value != value1 {
		t.Errorf("invalid value, want: %s, got: %s", value1, metadata.Value)
	}

	metadata, ok = requests[0].Metadata[1].(*hc.Variable)
	if !ok {
		t.Fatal("metadata[1] is not variable")
	}
	if metadata.Name != key2 {
		t.Errorf("invalid name, want: %s, got: %s", key2, metadata.Name)
	}
	if metadata.Value != value2 {
		t.Errorf("invalid value, want: %s, got: %s", value2, metadata.Value)
	}
}
