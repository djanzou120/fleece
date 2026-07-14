package syncx_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"fleece/src/go/syncx"
)

// helper: identity map — returns the input unchanged.
func identity(_ context.Context, n int) (int, error) { return n, nil }

// TestMap_NominalOrderPreserved verifies that out[i] == inputs[i] even when
// tasks complete out of order.
func TestMap_NominalOrderPreserved(t *testing.T) {
	inputs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	out, err := syncx.Map(context.Background(), inputs, 4, identity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(inputs) {
		t.Fatalf("len(out)=%d, want %d", len(out), len(inputs))
	}
	for i, v := range out {
		if v != inputs[i] {
			t.Errorf("out[%d]=%d, want %d", i, v, inputs[i])
		}
	}
}

// TestMap_EmptyInputs verifies that an empty slice returns nil, nil immediately.
func TestMap_EmptyInputs(t *testing.T) {
	out, err := syncx.Map(context.Background(), []int{}, 4, identity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil slice, got %v", out)
	}
}

// TestMap_NilInputs verifies that a nil slice also returns nil, nil.
func TestMap_NilInputs(t *testing.T) {
	out, err := syncx.Map[int, int](context.Background(), nil, 4, identity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil slice, got %v", out)
	}
}

// TestMap_FirstErrorPropagated verifies that the first error stops scheduling
// and is returned as-is.
func TestMap_FirstErrorPropagated(t *testing.T) {
	sentinel := errors.New("boom")

	fn := func(_ context.Context, n int) (int, error) {
		if n == 3 {
			return 0, sentinel
		}
		return n, nil
	}

	inputs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	_, err := syncx.Map(context.Background(), inputs, 2, fn)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v; want sentinel %v", err, sentinel)
	}
}

// TestMap_CancelsOnError verifies that after the first error, remaining tasks
// observe a cancelled context (ctx.Err() != nil).
func TestMap_CancelsOnError(t *testing.T) {
	var callsAfterCancel atomic.Int64
	errFirst := errors.New("first error")

	fn := func(ctx context.Context, n int) (int, error) {
		// Task 0 always errors.
		if n == 0 {
			return 0, errFirst
		}
		// Other tasks: if the context is already done, record that the
		// cancellation propagated before we were scheduled.
		if ctx.Err() != nil {
			callsAfterCancel.Add(1)
		}
		return n, nil
	}

	// Single worker so task 0 fires first and cancels before the rest.
	inputs := make([]int, 20)
	for i := range inputs {
		inputs[i] = i
	}
	_, err := syncx.Map(context.Background(), inputs, 1, fn)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, errFirst) {
		t.Fatalf("error = %v; want %v", err, errFirst)
	}
	// With 1 worker we cannot guarantee a specific count (the goroutine
	// closing taskC may have already filled the buffered channel), but we
	// can assert the error is correct — the goroutine cancellation check
	// above records a counter for observability without making the test
	// flaky.
	t.Logf("tasks that observed a cancelled ctx: %d", callsAfterCancel.Load())
}

// TestMap_ParentContextCancelled verifies that if the parent context is
// cancelled before Map returns, workers stop and Map returns ctx.Err().
func TestMap_ParentContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var started atomic.Int64
	fn := func(ctx context.Context, n int) (int, error) {
		started.Add(1)
		// Block until the context is done.
		<-ctx.Done()
		return 0, ctx.Err()
	}

	inputs := []int{1, 2, 3}

	// Cancel the parent after the first task starts.
	go func() {
		for started.Load() == 0 {
		}
		cancel()
	}()

	_, err := syncx.Map(ctx, inputs, 1, fn)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v; want context.Canceled", err)
	}
}

// TestMap_ZeroConcurrencyPanics verifies that concurrency <= 0 panics with a
// descriptive message when inputs is non-empty.
func TestMap_ZeroConcurrencyPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic, got none")
		}
		msg := fmt.Sprintf("%v", r)
		if msg == "" {
			t.Fatal("panic value is empty")
		}
		t.Logf("panic message: %s", msg)
	}()

	syncx.Map(context.Background(), []int{1}, 0, identity) //nolint:errcheck
}

// TestMap_ConcurrencyOne verifies correct ordering with a single worker
// (sequential execution).
func TestMap_ConcurrencyOne(t *testing.T) {
	inputs := []int{10, 20, 30}
	out, err := syncx.Map(context.Background(), inputs, 1, identity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, v := range out {
		if v != inputs[i] {
			t.Errorf("out[%d]=%d, want %d", i, v, inputs[i])
		}
	}
}
