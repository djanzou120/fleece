// Package syncx provides concurrency helpers for Go services.
//
// It is a transverse library (no business rules, stdlib-only).
package syncx

import (
	"context"
	"fmt"
	"sync"
)

// Map runs fn concurrently over inputs using at most concurrency workers.
//
// Ordering guarantee: out[i] corresponds to inputs[i].
//
// Early-termination: the first error returned by any invocation of fn cancels
// the shared child context and stops scheduling new tasks; that error is
// returned.  Results already computed before cancellation are still placed in
// their correct positions in the output slice, but the caller should treat the
// returned slice as partial when err != nil.
//
// Panics if concurrency <= 0 and len(inputs) > 0.
func Map[Input, Output any](
	ctx context.Context,
	inputs []Input,
	concurrency int,
	fn func(context.Context, Input) (Output, error),
) ([]Output, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	if concurrency <= 0 {
		panic(fmt.Errorf("syncx.Map: %d inputs with no workers", len(inputs)))
	}

	type task struct {
		i   int
		in  Input
		out Output
		err error
	}

	// Producer: send tasks into the pipeline.
	taskC := make(chan *task, concurrency)
	go func() {
		defer close(taskC)
		for i, item := range inputs {
			taskC <- &task{i: i, in: item}
		}
	}()

	// Child context so workers can be cancelled on first error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	resultsC := make(chan *task, concurrency)

	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case m, ok := <-taskC:
					if !ok {
						return
					}
					m.out, m.err = fn(ctx, m.in)
					resultsC <- m
				}
			}
		}()
	}

	// Close resultsC once all workers are done.
	go func() {
		wg.Wait()
		close(resultsC)
	}()

	out := make([]Output, len(inputs))
	var firstErr error
	for m := range resultsC {
		out[m.i] = m.out
		if m.err != nil && firstErr == nil {
			firstErr = m.err
			cancel()
		}
	}
	return out, firstErr
}
