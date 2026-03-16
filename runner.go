package hc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skranpn/hc/jsonpath"
	"github.com/skranpn/hc/metadata"
)

type RunnerOption func(*Runner)

func SetStopOnFailure(v bool) RunnerOption {
	return func(r *Runner) {
		r.stopOnFailure = v
	}
}
func SetStopOnError(v bool) RunnerOption {
	return func(r *Runner) {
		r.stopOnError = v
	}
}
func SetRequestTimeout(t int) RunnerOption {
	return func(r *Runner) {
		r.timeout = time.Duration(t) * time.Second
	}
}
func SetInterval(t int) RunnerOption {
	return func(r *Runner) {
		r.interval = time.Duration(t) * time.Millisecond
	}
}

type Runner struct {
	client   HttpClient
	vm       *VariableManager
	reporter *Reporter
	pauseCtl *PauseController

	stopOnFailure bool
	stopOnError   bool
	timeout       time.Duration
	interval      time.Duration
}

func NewRunner(client HttpClient, vm *VariableManager, reporter *Reporter, pauseCtl *PauseController, opts ...RunnerOption) *Runner {
	runner := &Runner{
		client:   client,
		vm:       vm,
		reporter: reporter,
		pauseCtl: pauseCtl,
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

// RunWithContext executes a request with a given context
func (r *Runner) RunWithContext(ctx context.Context, req *HttpRequest) (err error) {
	defer func() {
		r.reporter.Stderr(err)
	}()

	//  Variable substitution
	r.replaceVariables(req)

	err = r.handlePreRequest(req)
	if errors.Is(err, ErrSkip) {
		r.reporter.Stdout(req, nil, req.Metadata)
		r.reporter.Summary(req, nil, req.Metadata)
		return nil
	}

	// skip したら sleep はなし
	defer func() {
		if r.interval > 0 {
			sleep(ctx, r.interval)
		}
	}()

	for {
		resp, err := r.send(ctx, req)
		defer func() {
			r.reporter.Summary(req, resp, req.Metadata)
		}()
		if err != nil {
			// context canceled なら終了
			if errors.Is(err, contextCanceled) {
				return err
			}
			// stopOnXXX でも停止
			if r.stopOnFailure || r.stopOnError {
				return err
			}
			// それ以外なら継続 (エラーを無視)
			return fmt.Errorf("%w%v", ErrIgnorable, err)
		}

		// handleMetadata
		err = r.handleMetadata(ctx, req, resp)
		// 成功したら終了
		if err == nil {
			return nil
		}
		// context canceled なら終了
		if errors.Is(err, contextCanceled) {
			return err
		}
		// until のエラーならループ
		if _, ok := errors.AsType[*ErrUntilAssert](err); ok {
			continue
		}
		// それ以外のエラーで stopOnXXX なら終了
		if r.stopOnFailure || r.stopOnError {
			return err
		}
		// until 実行回数超えてたら終了
		if errors.Is(err, ErrUntilExceedMaximumRetry) {
			return nil
		}
	}
}

func (r *Runner) replaceVariables(req *HttpRequest) {
	defer func() {
		r.reporter.Variable(r.vm.variables)
	}()

	req.URL = r.vm.ReplaceVariables(req.URL)
	for k, v := range req.Headers {
		req.Headers[k] = r.vm.ReplaceVariables(v)
	}
	req.Body = r.vm.ReplaceVariables(req.Body)

	for _, m := range req.Metadata {
		m.Match(metadata.Cases{
			Assertion: func(a *metadata.Assertion) error {
				a.LeftPath = r.vm.ReplaceVariables(a.LeftPath)
				a.RightValue = r.vm.ReplaceVariables(a.RightValue)
				return nil
			},
			Until: func(u *metadata.Until) error {
				u.Condition.LeftPath = r.vm.ReplaceVariables(u.Condition.LeftPath)
				u.Condition.RightValue = r.vm.ReplaceVariables(u.Condition.RightValue)
				return nil
			},
			Skip: func(s *metadata.Skip) error {
				s.Condition.LeftPath = r.vm.ReplaceVariables(s.Condition.LeftPath)
				s.Condition.RightValue = r.vm.ReplaceVariables(s.Condition.RightValue)
				return nil
			},
			Variable: func(v *metadata.Variable) error { return nil },
		})
	}
}

func (r *Runner) send(ctx context.Context, req *HttpRequest) (*HttpResponse, error) {
	r.pauseCtl.WaitIfPaused()

	if err := ctx.Err(); err != nil {
		return nil, contextCanceled
	}

	// Create timeout context for individual request if not specified
	reqCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		if r.timeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, r.timeout)
			defer cancel()
		}
	}

	// Send request
	resp, err := r.client.Send(reqCtx, req)
	r.reporter.Result(req, resp, err)
	r.reporter.Current(req, resp, err)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (r *Runner) handlePreRequest(req *HttpRequest) (err error) {
	for _, m := range req.Metadata {
		err := m.Match(metadata.Cases{
			Skip: func(s *metadata.Skip) error {
				if ok, _ := s.Condition.Evaluate(""); ok {
					return ErrSkip
				}
				return nil
			},
			Assertion: func(a *metadata.Assertion) error { return nil },
			Until:     func(u *metadata.Until) error { return nil },
			Variable:  func(v *metadata.Variable) error { return nil },
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) handleMetadata(ctx context.Context, req *HttpRequest, resp *HttpResponse) (err error) {
	// すべてのリクエストの Metadata を処理し終わってから &&
	// until interval (sleep) よりも先に Stdout 出力をしたいので defer の順番はこう
	defer func() {
		if until, ok := errors.AsType[*ErrUntilAssert](err); ok {
			sleep(ctx, until.Interval)
		}
	}()
	defer func() {
		err = errors.Join(err,
			// r.reporter.Summary(req, resp, req.Metadata),
			r.reporter.Stdout(req, resp, req.Metadata),
		)
	}()

	unifiedJSON := resp.buildUnifiedJSON()

	for _, m := range req.Metadata {

		err = m.Match(metadata.Cases{
			Assertion: func(a *metadata.Assertion) error {
				ok, err := a.Evaluate(unifiedJSON)
				if err != nil && (r.stopOnFailure || r.stopOnError) {
					return err
				}
				r.reporter.Stderr(err)

				if !ok && r.stopOnFailure {
					return fmt.Errorf("assertion failed: %s", a.Raw)
				}

				return nil
			},
			Until: func(u *metadata.Until) error {
				u.CurrentAttempt++

				ok, err := u.Condition.Evaluate(unifiedJSON)
				if err != nil && (r.stopOnFailure || r.stopOnError) {
					return err
				}
				r.reporter.Stderr(err)

				// 成功したら抜ける
				if ok {
					return nil
				}

				// 実行回数チェック
				if u.CurrentAttempt >= u.MaxRetry {
					if r.stopOnFailure {
						return fmt.Errorf("until assertion failed: %s", u.Raw)
					}

					return ErrUntilExceedMaximumRetry
				}

				return &ErrUntilAssert{
					Interval: u.Interval,
				}
			},
			Variable: func(v *metadata.Variable) error {

				// jsonpath 内にネストした変数展開があるかもなので、事前に Replace する
				v.Value = r.vm.ReplaceVariables(v.Value)
				jsonpaths := v.JSONPaths()
				for i, jp := range v.JSONPaths() {
					jsonpaths[i] = r.vm.ReplaceVariables(jp)
				}

				// レスポンスと jsonpath を見て値を取得
				var values map[string]any
				values, err = jsonpath.All(unifiedJSON, jsonpaths)
				if err != nil && (r.stopOnFailure || r.stopOnError) {
					return err
				}
				r.reporter.Stderr(err)

				// jsonpath 変数に保存
				r.vm.SetJSONPaths(values)

				// 変数に保存
				r.vm.Set(v.Name, fmt.Sprintf("%v", v.Value))

				return nil
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func sleep(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return contextCanceled
		case <-timer.C:
			return nil
		}
	}
}
