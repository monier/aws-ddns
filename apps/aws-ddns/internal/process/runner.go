package process

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"
)

// Runner is the daemon entry point: it runs one synchronization cycle at
// startup, then repeats at the configured interval until the context is
// cancelled. A failing — or even panicking — cycle is logged and the loop
// keeps running.
type Runner struct {
	synchronizer Synchronizer
	interval     time.Duration
	logger       *slog.Logger
	cycle        int64
}

func NewRunner(synchronizer Synchronizer, interval time.Duration, logger *slog.Logger) *Runner {
	return &Runner{synchronizer: synchronizer, interval: interval, logger: logger}
}

// Run blocks until ctx is cancelled, then returns after a graceful stop.
func (r *Runner) Run(ctx context.Context) {
	r.runCycle(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("shutdown signal received, stopping")
			return
		case <-ticker.C:
			r.runCycle(ctx)
		}
	}
}

func (r *Runner) runCycle(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	r.cycle++
	logger := r.logger.With("cycle", r.cycle)
	start := time.Now()

	// A panic must not kill the daemon: record it with its stack and let the
	// next tick try again.
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error("synchronization cycle panicked, the loop keeps running",
				"panic", rec,
				"duration", time.Since(start).String(),
				"stack", string(debug.Stack()),
			)
		}
	}()

	logger.Info("synchronization cycle started")
	if err := r.synchronizer.Sync(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		logger.Error("synchronization cycle failed",
			"error", err,
			"duration", time.Since(start).String(),
			"nextRetryIn", r.interval.String(),
		)
		return
	}
	logger.Info("synchronization cycle completed",
		"duration", time.Since(start).String(),
		"nextCheckIn", r.interval.String(),
	)
}
