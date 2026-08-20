package process

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type countingSynchronizer struct {
	calls atomic.Int64
	err   error
}

func (c *countingSynchronizer) Sync(context.Context) error {
	c.calls.Add(1)
	return c.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func runFor(runner *Runner, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	runner.Run(ctx)
}

func TestRunSynchronizesImmediatelyAtStartup(t *testing.T) {
	synchronizer := &countingSynchronizer{}
	runner := NewRunner(synchronizer, time.Hour, discardLogger())

	runFor(runner, 50*time.Millisecond)

	assert.Equal(t, int64(1), synchronizer.calls.Load())
}

func TestRunRepeatsAtTheConfiguredInterval(t *testing.T) {
	synchronizer := &countingSynchronizer{}
	runner := NewRunner(synchronizer, 20*time.Millisecond, discardLogger())

	runFor(runner, 110*time.Millisecond)

	assert.GreaterOrEqual(t, synchronizer.calls.Load(), int64(3))
}

func TestRunKeepsRunningAfterTransientFailures(t *testing.T) {
	synchronizer := &countingSynchronizer{err: errors.New("transient AWS failure")}
	runner := NewRunner(synchronizer, 20*time.Millisecond, discardLogger())

	runFor(runner, 110*time.Millisecond)

	assert.GreaterOrEqual(t, synchronizer.calls.Load(), int64(3))
}

type panickingSynchronizer struct {
	calls atomic.Int64
}

func (p *panickingSynchronizer) Sync(context.Context) error {
	p.calls.Add(1)
	panic("boom")
}

func TestRunKeepsRunningAfterAPanickingCycle(t *testing.T) {
	synchronizer := &panickingSynchronizer{}
	runner := NewRunner(synchronizer, 20*time.Millisecond, discardLogger())

	runFor(runner, 110*time.Millisecond)

	// Every cycle panicked, yet the loop kept ticking instead of crashing.
	assert.GreaterOrEqual(t, synchronizer.calls.Load(), int64(3))
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	synchronizer := &countingSynchronizer{}
	runner := NewRunner(synchronizer, time.Hour, discardLogger())

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		runner.Run(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runner did not stop after context cancellation")
	}
}
