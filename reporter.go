package hc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/skranpn/hc/metadata"
)

type Report struct {
	Req *HttpRequest
	Res *HttpResponse
	Err error

	// 変数出力のため
	// nil じゃないときだけメソッドを呼ぶ
	Variable map[string]string
}

type summary struct {
	method     string
	url        string
	statusCode string
	status     string
	err        string
}

func (s summary) toMarkdownTable() string {
	return fmt.Sprintf("| %s | %s | %s | %s |", s.method, s.url, s.statusCode, s.status)
}

type Reporter struct {
	out string
	now time.Time

	summary []summary
}

func NewReporter(out string) *Reporter {
	return &Reporter{out: out, now: time.Now()}
}

func (r *Reporter) Start(ctx context.Context, ch <-chan *Report) {
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}

			var wg sync.WaitGroup
			wg.Add(5)
			go func() { defer wg.Done(); r.stdout(data) }()
			go func() { defer wg.Done(); r.stderr(data) }()
			go func() { defer wg.Done(); r.result(data) }()
			go func() { defer wg.Done(); r.variable(data) }()
			go func() { defer wg.Done(); r.save(data) }()
			wg.Wait()
		}
	}
}

func (r *Reporter) stdout(result *Report) error {
	// variable のみ、err のみ記録したいとき req が nil になりうる
	// この場合は何もせず終了
	if result.Req == nil {
		return nil
	}

	// skipped のとき resp が nil になりうる
	// req があれば記録はできるので継続

	indicator := color.New(color.FgRed, color.Bold)
	messages := make([]string, 0, len(result.Req.Metadata)+1)

	name := result.Req.Name
	if name == "" {
		name = fmt.Sprintf("%s %s", result.Req.Method, result.Req.URL)
	}
	if result.Res == nil {
		messages = append(messages, fmt.Sprintf("%s", name))
	} else {
		messages = append(messages, fmt.Sprintf("%d %s", result.Res.StatusCode, name))
	}

FOR:
	for _, m := range result.Req.Metadata {
		switch v := m.(type) {
		case *metadata.Skip:
			if v.Condition.Ok() {
				// if skipped, set yellow
				indicator = color.New(color.FgYellow, color.Bold)
				messages = append(messages, fmt.Sprintf("  └ %s, skipped", v.Condition.Raw))
				break
			}

		case *metadata.Until:
			messages = append(messages, fmt.Sprintf("  └ [%d/%d] %s: %s", v.CurrentAttempt, v.MaxRetry, v.Condition.Raw, v.Condition.StatusText()))
			if !v.IsFinish() {
				// if until condition is not met, set white
				indicator = color.New(color.FgWhite, color.Bold)

				break FOR
			}
		case *metadata.Assertion:
			messages = append(messages, fmt.Sprintf("  └ %s, %s", v.Raw, v.StatusText()))
			if v.Ok() {
				indicator = color.New(color.FgGreen, color.Bold)
			} else {
				indicator = color.New(color.FgRed, color.Bold)
			}
		}
	}

	if !result.Req.Metadata.Finish() {
		fmt.Fprint(os.Stdout, "\033[s")
		defer func() {
			fmt.Fprint(os.Stdout, "\033[u")
		}()
	}

	indicator.Print("● ")
	fmt.Fprint(os.Stdout, strings.Join(messages, "\r\n")+"\r\n")

	return nil
}

func (r *Reporter) stderr(result *Report) {
	if result.Err != nil {
		fmt.Fprintf(os.Stderr, "%+v\r\n", result.Err)
	}
}

// result も HTTP リクエスト、HTTP レスポンスをそのままファイルに出力します
// これまで実行したリクエスト、レスポンスがファイルに記録されます
func (r *Reporter) result(result *Report) error {
	// variable のみ、err のみ記録したいとき req が nil になりうる
	// この場合は何もせず終了
	if result.Req == nil {
		return nil
	}

	path, err := r.filepath("result", "txt")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(r.format(result.Req, result.Res, result.Err)))
	return err
}

func (r *Reporter) variable(result *Report) error {
	if result.Variable == nil {
		return nil
	}

	path, err := r.filepath("variable", "txt")
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range result.Variable {
		f.Write(fmt.Appendf(nil, "%s=%s\n", k, v))
	}

	return nil
}

func (r *Reporter) Flush() error {
	var f *os.File

	path, err := r.filepath("summary", "md")
	if err != nil {
		return err
	}
	f, err = os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	toMarkdownTable := func() []string {
		body := make([]string, 0, len(r.summary))
		for _, s := range r.summary {
			body = append(body, s.toMarkdownTable())
		}

		return body
	}

	body := toMarkdownTable()
	f.Write([]byte(strings.Join(
		append(
			[]string{
				"| Method | URL | StatusCode | Assert |",
				"|--- |--- |--- |--- |",
			},
			body...,
		),
		"\n",
	)))

	return nil
}

func (r *Reporter) save(result *Report) {

	if result.Req == nil {
		return
	}

	status := "N/A"
	if result.Req.Metadata != nil {
		status = result.Req.Metadata.Status()
	}

	statusCode := "N/A"
	if result.Res != nil {
		statusCode = fmt.Sprint(result.Res.StatusCode)
	}

	err := ""
	if result.Err != nil {
		err = result.Err.Error()
	}

	r.summary = append(r.summary, summary{
		method:     result.Req.Method,
		url:        result.Req.URL,
		statusCode: statusCode,
		status:     status,
		err:        err,
	})
}

func (r *Reporter) filepath(name string, ext string) (string, error) {
	dir := filepath.Join(r.out, fmt.Sprint(r.now.Unix()))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%s", name, ext)), nil
}

func (r *Reporter) format(req *HttpRequest, resp *HttpResponse, err error) string {
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s %s", req.Method, req.URL)
	}

	// request
	var reqHeaders strings.Builder
	for k, v := range req.Headers {
		fmt.Fprintf(&reqHeaders, "%s: %s\n", k, v)
	}
	request := fmt.Sprintf("%s %s\n%s\n%s", req.Method, req.URL, reqHeaders.String(), string(req.Body))

	// response
	var response string
	if err != nil {
		response = err.Error()
	} else if resp == nil {
		response = "response is null"
	} else {
		var respHeaders strings.Builder
		for k, v := range resp.Header {
			fmt.Fprintf(&respHeaders, "%s: %s\n", k, v[0])
		}
		response = fmt.Sprintf("%s %d %s\n%s\n%s", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode), respHeaders.String(), string(resp.Body))
	}

	return fmt.Sprintf("## %s\n\n%s\n\n%s\n\n", name, request, response)
}
