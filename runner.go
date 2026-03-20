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

type Runner struct {
	client   HttpClient
	vm       *VariableManager
	pauseCtl *PauseController

	ch chan<- *Report

	stopOnFailure bool
	stopOnError   bool
	timeout       time.Duration
}

func NewRunner(client HttpClient, vm *VariableManager, pauseCtl *PauseController, ch chan<- *Report, opts ...RunnerOption) *Runner {
	runner := &Runner{
		client:   client,
		vm:       vm,
		pauseCtl: pauseCtl,
		ch:       ch,
	}

	for _, opt := range opts {
		opt(runner)
	}

	return runner
}

// RunWithContext executes a request with a given context
func (r *Runner) RunWithContext(ctx context.Context, req *HttpRequest) (err error) {
	//  Variable substitution
	r.replaceVariables(req)

	// skip のチェックは request interval の前にする
	err = r.handlePreRequest(req)
	if errors.Is(err, ErrSkip) {
		r.ch <- &Report{Req: req}
		return nil
	}

	for {
		err := r.run(ctx, req)
		// 成功したら終了
		if err == nil {
			return nil
		}
		// context canceled なら終了
		if errors.Is(err, contextCanceled) {
			return err
		}
		// until のエラーならループ
		if until, ok := errors.AsType[*ErrUntilAssert](err); ok {
			sleep(ctx, until.Interval)
			continue
		}
		// 無視可能なエラーなら終了
		if errors.Is(err, ErrIgnorable) {
			return err
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

func (r *Runner) run(ctx context.Context, req *HttpRequest) (err error) {
	var resp *HttpResponse

	defer func() {
		e := err
		if _, ok := errors.AsType[*ErrUntilAssert](err); ok {
			e = nil
		}

		if ctx.Err() == nil {
			r.ch <- &Report{Req: req, Res: resp, Err: e}
		}
	}()

	resp, err = r.send(ctx, req)
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
	return r.handleMetadata(ctx, req, resp)
}

func (r *Runner) replaceVariables(req *HttpRequest) {
	defer func() {
		r.ch <- &Report{Variable: r.vm.variables}
	}()

	req.URL = r.vm.ReplaceVariables(req.URL)
	for k, v := range req.Headers {
		req.Headers[k] = r.vm.ReplaceVariables(v)
	}
	req.Body = r.vm.ReplaceVariables(req.Body)

	for _, m := range req.Metadata {
		switch v := m.(type) {
		case *metadata.Assertion:
			v.LeftPath = r.vm.ReplaceVariables(v.LeftPath)
			v.RightValue = r.vm.ReplaceVariables(v.RightValue)
		case *metadata.Until:
			v.Condition.LeftPath = r.vm.ReplaceVariables(v.Condition.LeftPath)
			v.Condition.RightValue = r.vm.ReplaceVariables(v.Condition.RightValue)
		case *metadata.Skip:
			v.Condition.LeftPath = r.vm.ReplaceVariables(v.Condition.LeftPath)
			v.Condition.RightValue = r.vm.ReplaceVariables(v.Condition.RightValue)
		}
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
	return r.client.Send(reqCtx, req)
}

func (r *Runner) handlePreRequest(req *HttpRequest) (err error) {
	for _, m := range req.Metadata {
		switch s := m.(type) {
		case *metadata.Skip:
			if ok, _ := s.Condition.Evaluate(""); ok {
				return ErrSkip
			}
		}
	}

	return nil
}

func (r *Runner) handleMetadata(ctx context.Context, req *HttpRequest, resp *HttpResponse) (err error) {
	unifiedJSON := resp.buildUnifiedJSON()

	for _, m := range req.Metadata {

		switch v := m.(type) {
		case *metadata.Assertion:
			ok, err := v.Evaluate(unifiedJSON)
			if err != nil && (r.stopOnFailure || r.stopOnError) {
				return err
			}
			r.ch <- &Report{Err: err}

			if !ok && r.stopOnFailure {
				return fmt.Errorf("assertion failed: %s", v.Raw)
			}

		case *metadata.Until:
			v.CurrentAttempt++

			ok, err := v.Condition.Evaluate(unifiedJSON)
			if err != nil && (r.stopOnFailure || r.stopOnError) {
				return err
			}
			r.ch <- &Report{Err: err}

			// 成功したら抜ける
			if ok {
				return nil
			}

			if !ok {
				// 実行回数チェック
				if v.CurrentAttempt >= v.MaxRetry {
					if r.stopOnFailure {
						return fmt.Errorf("until assertion failed: %s", v.Raw)
					}

					return ErrUntilExceedMaximumRetry
				}

				return &ErrUntilAssert{
					Interval: v.Interval,
				}
			}

		case *metadata.Variable:

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
			r.ch <- &Report{Err: err}

			// jsonpath 変数に保存
			r.vm.SetJSONPaths(values)

			// 変数に保存
			r.vm.Set(v.Name, fmt.Sprintf("%v", v.Value))
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
