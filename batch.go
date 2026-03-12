package hc

import (
	"context"
	"errors"
	"maps"
	"regexp"

	"github.com/skranpn/hc/metadata"
	"golang.org/x/sync/errgroup"
)

// DependencyResolver analyzes request dependencies based on variable usage
type DependencyResolver struct {
	requests []HttpRequest
}

// NewDependencyResolver creates a new DependencyResolver
func NewDependencyResolver(requests []HttpRequest) *DependencyResolver {
	return &DependencyResolver{
		requests: requests,
	}
}

// BuildExecutionPlan analyzes dependencies and returns execution stages
// Each stage contains request indices that can be executed in parallel
func (dr *DependencyResolver) BuildExecutionPlan() [][]int {
	if len(dr.requests) == 0 {
		return [][]int{}
	}

	// Build dependency map: which requests depend on which
	dependencies := make(map[int]map[int]bool) // dependencies[i][j] = true means i depends on j
	definedVars := make(map[string]int)
	for i := range dr.requests {
		dependencies[i] = make(map[int]bool)
	}

	// Extract variable references from each request
	varRegex := regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)`)
	for i, req := range dr.requests {
		// Collect all variables used in this request
		usedVars := make(map[string]bool)
		for _, match := range varRegex.FindAllStringSubmatch(req.URL, -1) {
			if len(match) > 1 {
				usedVars[match[1]] = true
			}
		}
		for _, value := range req.Headers {
			for _, match := range varRegex.FindAllStringSubmatch(value, -1) {
				if len(match) > 1 {
					usedVars[match[1]] = true
				}
			}
		}
		for _, match := range varRegex.FindAllStringSubmatch(req.Body, -1) {
			if len(match) > 1 {
				usedVars[match[1]] = true
			}
		}

		// variable として保存したもの
		for _, m := range req.Metadata {
			switch v := m.(type) {
			case *metadata.Variable:
				definedVars[v.Name] = i
			}
		}

		// Find which previous requests set these variables
		for j := range i {
			// jsonpath を使った直接参照
			if dr.requests[j].Name != "" && usedVars[dr.requests[j].Name] {
				dependencies[i][j] = true
			}

			// 変数名を使った参照
			for v := range usedVars {
				if index, ok := definedVars[v]; ok {
					dependencies[i][index] = true
				}
			}
		}
	}

	// Topological sort to build execution plan
	return dr.topologicalSort(dependencies)
}

// topologicalSort performs topological sort to find parallelizable stages
func (dr *DependencyResolver) topologicalSort(dependencies map[int]map[int]bool) [][]int {
	var stages [][]int
	processed := make(map[int]bool)
	remaining := make(map[int]map[int]bool)

	// Deep copy dependencies
	for k, v := range dependencies {
		remaining[k] = make(map[int]bool)
		maps.Copy(remaining[k], v)
	}

	for len(processed) < len(dr.requests) {
		// Find requests with no remaining dependencies
		var stage []int
		for i := 0; i < len(dr.requests); i++ {
			if processed[i] {
				continue
			}
			if len(remaining[i]) == 0 {
				stage = append(stage, i)
				processed[i] = true
			}
		}

		if len(stage) == 0 {
			// No stage found - may have circular dependencies
			// Fall back to sequential execution
			var seq []int
			for i := 0; i < len(dr.requests); i++ {
				if !processed[i] {
					seq = append(seq, i)
					processed[i] = true
				}
			}
			if len(seq) > 0 {
				stages = append(stages, seq)
			}
			break
		}

		stages = append(stages, stage)

		// Remove processed items from remaining dependencies
		for i := range remaining {
			for _, p := range stage {
				delete(remaining[i], p)
			}
		}
	}

	return stages
}

// Batch executes requests in batches (potentially in parallel)
type Batch interface {
	Execute(ctx context.Context, requests []HttpRequest, runner *Runner) error
}

// SequentialBatch executes requests sequentially
type SequentialBatch struct{}

// Execute runs requests sequentially
func (sbe *SequentialBatch) Execute(ctx context.Context, requests []HttpRequest, runner *Runner) error {
	for _, req := range requests {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := runner.RunWithContext(ctx, &req); err != nil {
			return err
		}
	}
	return nil
}

// ParallelBatch executes requests in parallel stages
type ParallelBatch struct {
	maxConcurrency int
}

// NewParallelBatch creates a new ParallelBatchExecutor
func NewParallelBatch(maxConcurrency int) *ParallelBatch {
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	return &ParallelBatch{
		maxConcurrency: maxConcurrency,
	}
}

// Execute runs requests with parallel stages
func (pbe *ParallelBatch) Execute(ctx context.Context, requests []HttpRequest, runner *Runner) error {
	resolver := NewDependencyResolver(requests)
	stages := resolver.BuildExecutionPlan()

	for _, stage := range stages {
		if err := ctx.Err(); err != nil {
			return err
		}

		eg, egCtx := errgroup.WithContext(ctx)
		eg.SetLimit(pbe.maxConcurrency)

		for _, idx := range stage {
			req := requests[idx]

			eg.Go(func() error {
				err := runner.RunWithContext(egCtx, &req)
				if !errors.Is(err, ErrIgnorable) {
					return err
				}
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	return nil
}

// batch is a convenience wrapper for batch execution
type batch struct {
	runner         *Runner
	executor       Batch
	parallelMode   bool
	maxConcurrency int
}

// NewBatch creates a new BatchRunner
func NewBatch(runner *Runner, parallelMode bool, maxConcurrency int) *batch {
	var executor Batch
	if parallelMode {
		executor = NewParallelBatch(maxConcurrency)
	} else {
		executor = &SequentialBatch{}
	}

	return &batch{
		runner:         runner,
		executor:       executor,
		parallelMode:   parallelMode,
		maxConcurrency: maxConcurrency,
	}
}

// Run executes multiple requests using the configured executor
func (br *batch) Run(ctx context.Context, requests []HttpRequest) error {
	if len(requests) == 0 {
		return nil
	}

	return br.executor.Execute(ctx, requests, br.runner)
}
