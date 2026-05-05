// Package txhelper owns transaction lifecycle and post-commit observer
// dispatch. It is a thin helper; business logic lives in service methods.
//
// # Post-commit queueing
//
// Inside the fn callback passed to Run, service methods call QueuePostCommit
// to register observer calls that fire after the transaction commits. The
// queue is stored on a context key that Run sets before invoking fn.
//
// Calling QueuePostCommit with a context that was not produced by Run
// (i.e. that carries no queue) panics. This is a programming error and
// fail-fast detection is preferable to silent no-ops that mask bugs.
//
// # Logger injection
//
// Post-commit dispatch errors are logged via the *slog.Logger provided to
// RunWithLogger. Run uses slog.Default(). Pass a custom logger in tests to
// capture log output.
package txhelper

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/moduleforge/core-api/observer"
)

// DB is the minimal interface txhelper needs from a connection pool.
// pgxpool.Pool satisfies it.
type DB interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// postCommitEntry holds a single queued post-commit observer call.
type postCommitEntry struct {
	obs            observer.MutationObserver
	op, resource   string
	targetEntityID *int64
	before, after  any
}

// queueKey is the unexported context key for the post-commit queue.
type queueKey struct{}

// Run executes fn inside a transaction. On fn success, the tx is committed
// and any post-commit observer calls queued via QueuePostCommit are dispatched
// in parallel. On fn error, the tx is rolled back and no post-commit observers
// fire.
//
// Post-commit dispatch errors are logged via slog.Default() and do not
// propagate to the caller.
func Run(
	ctx context.Context,
	db DB,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	return RunWithLogger(ctx, db, fn, nil)
}

// RunWithLogger is like Run but accepts a *slog.Logger for post-commit error
// logging. When logger is nil, slog.Default() is used.
func RunWithLogger(
	ctx context.Context,
	db DB,
	fn func(ctx context.Context, tx pgx.Tx) error,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("txhelper: begin tx: %w", err)
	}

	// Attach an empty queue to the context so QueuePostCommit can accumulate
	// entries inside fn.
	var queue []postCommitEntry
	innerCtx := context.WithValue(ctx, queueKey{}, &queue)

	if err := fn(innerCtx, tx); err != nil {
		// Roll back on any fn error; swallow rollback errors to preserve the
		// original error returned to the caller.
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("txhelper: commit: %w", err)
	}

	// Dispatch post-commit observers in parallel. Errors are logged only.
	dispatchPostCommit(ctx, queue, logger)
	return nil
}

// dispatchPostCommit fires all queued post-commit calls. Each observer runs
// in its own goroutine; errors are logged and not propagated.
func dispatchPostCommit(ctx context.Context, queue []postCommitEntry, logger *slog.Logger) {
	if len(queue) == 0 {
		return
	}

	// Simple parallel dispatch using goroutines; we do not need errgroup
	// here because we never propagate errors.
	done := make(chan struct{}, len(queue))
	for _, e := range queue {
		e := e
		go func() {
			defer func() { done <- struct{}{} }()
			if err := e.obs.ObserveAfterCommit(ctx, e.op, e.resource, e.targetEntityID, e.before, e.after); err != nil {
				logger.ErrorContext(ctx, "txhelper: post-commit observer error (swallowed)",
					"op", e.op, "resource", e.resource, "error", err)
			}
		}()
	}
	for range queue {
		<-done
	}
}

// QueuePostCommit queues a post-commit dispatch from inside fn. Typically
// called by service methods after the in-tx Observe call. The queue is stored
// on ctx, so callers must pass the ctx that Run provides to fn.
//
// Calling QueuePostCommit with a context not produced by Run panics. This is a
// programming error; fail-fast detection is preferable to silent no-ops.
func QueuePostCommit(
	ctx context.Context,
	obs observer.MutationObserver,
	op, resource string,
	targetEntityID *int64,
	before, after any,
) {
	q, ok := ctx.Value(queueKey{}).(*[]postCommitEntry)
	if !ok || q == nil {
		panic("txhelper.QueuePostCommit called with a context not produced by txhelper.Run")
	}
	*q = append(*q, postCommitEntry{
		obs:            obs,
		op:             op,
		resource:       resource,
		targetEntityID: targetEntityID,
		before:         before,
		after:          after,
	})
}
