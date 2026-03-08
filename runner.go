package hc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skranpn/hc/jsonpath"
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
		r.timeout = t
	}
}

type Runner struct {
	client   HttpClient
	vm       *VariableManager
	reporter *reporter
	pauseCtl *PauseController

	stopOnFailure bool
	stopOnError   bool
	timeout       int
}

func NewRunner(client HttpClient, vm *VariableManager, reporter *reporter, pauseCtl *PauseController, opts ...RunnerOption) *Runner {
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
	req.URL = r.vm.ReplaceVariables(req.URL)
	for k, v := range req.Headers {
		req.Headers[k] = r.vm.ReplaceVariables(v)
	}
	req.Body = r.vm.ReplaceVariables(req.Body)

	// send request
	resp, err := r.run(ctx, req)
	if err != nil {
		if errors.Is(err, contextCanceled) {
			return err
		}
		if r.stopOnFailure || r.stopOnError {
			return err
		}
	}

	// 実行結果の出力
	defer func() {
		err = errors.Join(err,
			r.reporter.Result(req, resp),
			r.reporter.Variable(r.vm.variables),
		)
	}()

	err = r.handleMetadata(ctx, req, resp)
	if errors.Is(err, contextCanceled) {
		return err
	}
	if err != nil && (r.stopOnFailure || r.stopOnError) {
		return err
	}

	return nil
}

func (r *Runner) run(ctx context.Context, req *HttpRequest) (*HttpResponse, error) {
	r.pauseCtl.WaitIfPaused()

	if err := ctx.Err(); err != nil {
		return nil, contextCanceled
	}

	// Create timeout context for individual request if not specified
	reqCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		if r.timeout > 0 {
			reqCtx, cancel = context.WithTimeout(ctx, time.Duration(r.timeout)*time.Second)
			defer cancel()
		}
	}

	// Send request
	resp, err := r.client.Send(reqCtx, req)
	if err != nil {
		return nil, err
	}
	r.reporter.Current(req, resp)

	return resp, err
}

func (r *Runner) handleMetadata(ctx context.Context, req *HttpRequest, resp *HttpResponse) (err error) {
	unifiedJSON := resp.buildUnifiedJSON()

	// 出力に metadata を含むのでここ
	defer func() {
		err = errors.Join(err,
			r.reporter.Summary(req, resp, req.Metadata),
			r.reporter.Stdout(req, resp, req.Metadata),
		)
	}()

	for _, metadata := range req.Metadata {
		switch v := metadata.(type) {
		case *Until:
			defer func(metadata *Until) {
				metadata.Finish = true
			}(v)

			for v.CurrentAttempt = 1; v.CurrentAttempt < v.MaxRetry; v.CurrentAttempt++ {
				_, err = v.Condition.Evaluate(resp, unifiedJSON)
				if err != nil && (r.stopOnFailure || r.stopOnError) {
					return err
				}
				r.reporter.Stderr(err)

				// 成功したら抜ける
				if v.Condition.Ok() {
					break
				}

				// 失敗したら次のリクエスト送信前に出力する
				err = errors.Join(err,
					r.reporter.Summary(req, resp, MetadataSlice{v}),
					r.reporter.Stdout(req, resp, MetadataSlice{v}),
				)
				r.reporter.Stderr(err)

				time.Sleep(v.Interval)

				// run again
				resp, err := r.run(ctx, req)
				if errors.Is(err, contextCanceled) {
					return err
				}
				if err != nil && (r.stopOnFailure || r.stopOnError) {
					return err
				}
				r.reporter.Stderr(err)

				// StopOnXXX を有効にしていないとき、resp が nil でここに来る可能性がある
				// チェックしてから build する
				if err == nil {
					unifiedJSON = resp.buildUnifiedJSON()
				}
			}

			if v.CurrentAttempt == v.MaxRetry && r.stopOnFailure {
				return fmt.Errorf("until assertion failed: %s", v.Raw)
			}

		case *Assertion:
			_, err = v.Evaluate(resp, unifiedJSON)
			if err != nil && (r.stopOnFailure || r.stopOnError) {
				return err
			}
			r.reporter.Stderr(err)

			if v.NG && r.stopOnFailure {
				return fmt.Errorf("assertion failed: %s", v.Raw)
			}

		case *Variable:
			var values map[string]any
			values, err = jsonpath.All(unifiedJSON, v.JSONPaths())
			if err != nil && (r.stopOnFailure || r.stopOnError) {
				return err
			}
			r.reporter.Stderr(err)

			r.vm.Set(v.Name, fmt.Sprintf("%v", v.Value), values)
		}
	}

	return nil
}
