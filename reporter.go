package hc

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
)

type reporter struct {
	out string
	now time.Time
}

func NewReporter(out string) *reporter {
	return &reporter{
		out: out,
		now: time.Now(),
	}
}

// Stdout はリクエストの実行結果が標準出力に書かれます
func (r *reporter) Stdout(req *HttpRequest, resp *HttpResponse, metadata MetadataSlice) (err error) {
	if !metadata.Finish() {
		fmt.Fprint(os.Stdout, "\033[s")
		defer func() {
			_, err = fmt.Fprint(os.Stdout, "\033[u")
		}()
	}

	if metadata.OK() {
		c := color.New(color.FgGreen, color.Bold)
		c.Print("● ")
	} else {
		c := color.New(color.FgRed, color.Bold)
		c.Print("● ")
	}

	fmt.Fprintf(os.Stdout, "%d %s\r\n", resp.StatusCode, req.Name)

	for _, m := range metadata {
		switch v := m.(type) {
		case *Until:
			fmt.Fprintf(os.Stdout, "  └ [%d/%d] %s: %s\r\n", v.CurrentAttempt, v.MaxRetry, v.Condition.Raw, v.Condition.Status())
		case *Assertion:
			fmt.Fprintf(os.Stdout, "  └ %s, %s\r\n", v.Raw, v.Status())
		}
	}

	return nil
}

// Stderr は実行中に起きたエラーが標準エラー出力に書かれます
func (r *reporter) Stderr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\r\n", err)
	}
}

// Current は HTTP リクエスト、HTTP レスポンスをそのままファイルに出力します
// 直前に実行したリクエスト、レスポンスのみがファイルに書かれます
func (r *reporter) Current(req *HttpRequest, resp *HttpResponse) error {
	path, err := r.filepath("last", "txt")
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(r.format(req, resp)))
	return err
}

// Result も HTTP リクエスト、HTTP レスポンスをそのままファイルに出力します
// これまで実行したリクエスト、レスポンスがファイルに記録されます
func (r *reporter) Result(req *HttpRequest, resp *HttpResponse) error {
	path, err := r.filepath("result", "txt")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(r.format(req, resp)))
	return err
}

// Summary は実行したリクエストの結果を markdown の表形式となるようにファイルに出力します
func (r *reporter) Summary(req *HttpRequest, resp *HttpResponse, metadata MetadataSlice) error {
	var f *os.File

	path, err := r.filepath("summary", "md")
	if err != nil {
		return err
	}
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		f.Write([]byte("| Method | URL | StatusCode | Assert |\n|--- |--- |--- |--- |\n"))
	} else {
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
	}

	status := ""
	if metadata != nil {
		status = metadata.Status()
	}
	_, err = f.Write(fmt.Appendf(nil, "| %s | %s | %d | %s |\n", req.Method, req.URL, resp.StatusCode, status))
	return err
}

// Variable は実行時に使った変数をファイルに出力します
func (r *reporter) Variable(variable map[string]string) error {
	path, err := r.filepath("variable", "txt")
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	for k, v := range variable {
		f.Write(fmt.Appendf(nil, "%s=%s\n", k, v))
	}

	return nil
}

func (r *reporter) filepath(name string, ext string) (string, error) {
	dir := filepath.Join(r.out, fmt.Sprint(r.now.Unix()))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%s", name, ext)), nil
}

func (r *reporter) format(req *HttpRequest, resp *HttpResponse) string {
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
	var respHeaders strings.Builder
	for k, v := range resp.Header {
		fmt.Fprintf(&respHeaders, "%s: %s\n", k, v[0])
	}
	response := fmt.Sprintf("%s %d %s\n%s\n%s", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode), respHeaders.String(), string(resp.Body))

	return fmt.Sprintf("## %s\n\n%s\n\n%s\n\n", name, request, response)
}
