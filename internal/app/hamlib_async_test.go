package app

import (
	"context"
	"testing"
	"time"
)

func TestHamlibExecSemaphoreHonorsCancellation(t *testing.T) {
	a := New()
	ctxHeld := context.Background()
	if err := a.acquireHamlibExec(ctxHeld); err != nil {
		t.Fatalf("initial acquire: %v", err)
	}
	defer a.releaseHamlibExec()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := a.acquireHamlibExec(ctx); err == nil {
		t.Fatal("second acquire unexpectedly succeeded while semaphore was held")
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("cancellation did not interrupt semaphore wait promptly: %v", elapsed)
	}
}

func TestCancelHamlibOperationsInvalidatesGeneration(t *testing.T) {
	a := New()
	before := a.hamlibProbeGeneration()
	ctx, done := a.beginHamlibOperation(time.Second)
	a.cancelHamlibOperations()
	defer done()
	if a.hamlibProbeCurrent(before) {
		t.Fatal("cancel did not invalidate the probe generation")
	}
	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("active Hamlib operation context was not cancelled")
	}
}
