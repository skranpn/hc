package hc

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/skranpn/hc/metadata"
)

type Reporter struct {
	out string
	now time.Time
}

func NewReporter(out string) *Reporter {
	return &Reporter{
		out: out,
		now: time.Now(),
	}
}

// Stdout はリクエストの実行結果が標準出力に書かれます
func (r Reporter) Stdout(req *HttpRequest, resp *HttpResponse, _metadata metadata.MetadataSlice) (err error) {
	if !_metadata.Finish() {
		fmt.Fprint(os.Stdout, "\033[s")
		defer func() {
			_, err = fmt.Fprint(os.Stdout, "\033[u")
		}()
	}

	if _metadata.Skipped() {
		c := color.New(color.FgYellow, color.Bold)
		c.Print("● ")
	} else if _metadata.OK() {
		c := color.New(color.FgGreen, color.Bold)
		c.Print("● ")
	} else {
		c := color.New(color.FgRed, color.Bold)
		c.Print("● ")
	}

	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s %s", req.Method, req.URL)
	}
	if resp == nil {
		fmt.Fprintf(os.Stdout, "%s\r\n", name)
	} else {
		fmt.Fprintf(os.Stdout, "%d %s\r\n", resp.StatusCode, name)
	}

	for _, m := range _metadata {
		m.Match(metadata.Cases{
			Until: func(u *metadata.Until) error {
				fmt.Fprintf(os.Stdout, "  └ [%d/%d] %s: %s\r\n", u.CurrentAttempt, u.MaxRetry, u.Condition.Raw, u.Condition.StatusText())
				return nil
			},
			Assertion: func(a *metadata.Assertion) error {
				fmt.Fprintf(os.Stdout, "  └ %s, %s\r\n", a.Raw, a.StatusText())
				return nil
			},
			Skip: func(s *metadata.Skip) error {
				fmt.Fprintf(os.Stdout, "  └ %s, skipped \r\n", s.Condition.Raw)
				return nil
			},
			Variable: func(v *metadata.Variable) error { return nil },
		})
	}

	return nil
}

// Stderr は実行中に起きたエラーが標準エラー出力に書かれます
func (r Reporter) Stderr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v\r\n", err)
	}
}

// Current は HTTP リクエスト、HTTP レスポンスをそのままファイルに出力します
// 直前に実行したリクエスト、レスポンスのみがファイルに書かれます
func (r Reporter) Current(req *HttpRequest, resp *HttpResponse, arg_err error) error {
	path, err := r.filepath("last", "txt")
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(r.format(req, resp, arg_err)))
	return err
}

// Result も HTTP リクエスト、HTTP レスポンスをそのままファイルに出力します
// これまで実行したリクエスト、レスポンスがファイルに記録されます
func (r Reporter) Result(req *HttpRequest, resp *HttpResponse, err error) error {
	path, err := r.filepath("result", "txt")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(r.format(req, resp, err)))
	return err
}

// Summary は実行したリクエストの結果を markdown の表形式となるようにファイルに出力します
func (r Reporter) Summary(req *HttpRequest, resp *HttpResponse, metadata metadata.MetadataSlice) error {
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
	statusCode := "N/A"
	if resp != nil {
		statusCode = fmt.Sprint(resp.StatusCode)
	}
	_, err = f.Write(fmt.Appendf(nil, "| %s | %s | %s | %s |\n", req.Method, req.URL, statusCode, status))
	return err
}

// Variable は実行時に使った変数をファイルに出力します
func (r Reporter) Variable(variable map[string]string) error {
	path, err := r.filepath("variable", "txt")
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range variable {
		f.Write(fmt.Appendf(nil, "%s=%s\n", k, v))
	}

	return nil
}

func (r Reporter) filepath(name string, ext string) (string, error) {
	dir := filepath.Join(r.out, fmt.Sprint(r.now.Unix()))
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%s", name, ext)), nil
}

func (r Reporter) format(req *HttpRequest, resp *HttpResponse, err error) string {
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
