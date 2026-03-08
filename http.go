package hc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HttpClient defines the interface for executing HTTP requests
type HttpClient interface {
	Send(ctx context.Context, req *HttpRequest) (*HttpResponse, error)
}

// httpClient is the implementation of RequestExecutor using net/http.Client
type httpClient struct {
	client *http.Client
}

// NewHttpClient creates a new HttpClientExecutor
func NewHttpClient(client *http.Client) *httpClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpClient{
		client: client,
	}
}

// Execute sends an HTTP request and returns the response
func (hce *httpClient) Send(ctx context.Context, req *HttpRequest) (*HttpResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if req.URL == "" {
		return nil, fmt.Errorf("request URL is empty")
	}

	if req.Method == "" {
		req.Method = http.MethodGet
	}

	// Create HTTP request with context
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("%s, failed to create request: %v", req.Name, err)
	}

	// Set headers
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	// Set body if present
	if req.Body != "" {
		httpReq.Body = io.NopCloser(strings.NewReader(req.Body))
	}

	// Execute request
	httpResp, err := hce.client.Do(httpReq)
	if err != nil {
		// Check if it's a context timeout/cancellation
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s, request timeout", req.Name)
		}

		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("%s, request cancelled", req.Name)
		}

		return nil, fmt.Errorf("%s, failed to execute request: %v", req.Name, err)
	}

	// Read response body
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s, failed to read response body: %v", req.Name, err)
	}

	return &HttpResponse{
		Proto:      httpResp.Proto,
		StatusCode: httpResp.StatusCode,
		Header:     httpResp.Header,
		Body:       body,
	}, nil
}

// FakeClient is a fake executor for testing
type FakeClient struct {
	responses map[string]*HttpResponse
	errors    map[string]error
}

// NewFakeClient creates a new FakeExecutor
func NewFakeClient() *FakeClient {
	return &FakeClient{
		responses: make(map[string]*HttpResponse),
		errors:    make(map[string]error),
	}
}

// Setup sets up a fake response for a URL
func (me *FakeClient) Setup(url string, resp *HttpResponse) {
	me.responses[url] = resp
}

// SetupError sets up an error for a URL
func (me *FakeClient) SetupError(url string, err error) {
	me.errors[url] = err
}

// Execute returns a fakeed response or error
func (me *FakeClient) Send(ctx context.Context, req *HttpRequest) (*HttpResponse, error) {
	if err, ok := me.errors[req.URL]; ok {
		return nil, err
	}

	if resp, ok := me.responses[req.URL]; ok {
		return resp, nil
	}

	return nil, fmt.Errorf("%s, no fake response set for URL: %s", req.Name, req.URL)

}

type HttpRequest struct {
	Name     string
	Method   string
	URL      string
	Headers  map[string]string
	Body     string
	Metadata []Metadata
}

type HttpResponse struct {
	Proto      string
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (r *HttpResponse) buildUnifiedJSON() string {
	// 1. ボディの読み込み
	// 2. ボディを JSON としてパース (JSONでない場合は文字列として扱う)
	var body any
	if err := json.Unmarshal(r.Body, &body); err != nil {
		// JSONとしてパースできない場合は、生の文字列として保持
		body = string(r.Body)
	}

	// 3. ヘッダーを map に変換
	headers := make(map[string]string)
	for k, v := range r.Header {
		headers[k] = v[0]
	}

	d := map[string]any{
		"response": map[string]any{
			"status":  r.StatusCode,
			"headers": headers,
			"body":    body,
		},
	}

	// 5. @assert などから使えるように JSON 文字列を生成
	unifiedBytes, _ := json.Marshal(d)
	return string(unifiedBytes)
}
