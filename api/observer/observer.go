// Package observer defines the MutationObserver interface and the ObserverGroup
// concrete type for fan-out dispatch to multiple observers.
//
// # Interfaces vs concrete types
//
// Service methods receive *ObserverGroup (not the bare MutationObserver
// interface) so they can choose Observe / MustObserve / MayObserve per call
// site without sacrificing the flexibility of a per-group default policy.
//
// # Logger injection
//
// Functions that must log errors (MayObserve, ObserveAfterCommit) accept an
// optional *slog.Logger via ObserverGroup.WithLogger. When nil (the default),
// slog.Default() is used at each call site. This keeps the zero-value
// ObserverGroup valid without any initialisation, while still allowing test
// code to capture log output by passing in a custom logger.
package observer

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

// MutationObserver is implemented by audit-module, outbox-module,
// cache-invalidator, search-index-sync, and any other consumer that wants
// to react to entity mutations. Implementations return errors honestly;
// the decision to swallow or propagate is made by the wrapping ObserverGroup
// based on its configured Policy and the call variant (Observe / MustObserve
// / MayObserve) the service method uses.
//
// An implementation may implement only one of the two methods (the other
// becoming a no-op returning nil) if it only cares about one phase.
type MutationObserver interface {
	// Observe runs inside the operation's transaction. The error return is
	// passed to the calling ObserverGroup, which decides whether to
	// propagate or log-and-swallow based on its configured Policy and the
	// call variant in use.
	Observe(
		ctx context.Context,
		tx pgx.Tx,
		op string,
		resource string,
		targetEntityID *int64,
		before, after any,
	) error

	// ObserveAfterCommit runs after the tx has successfully committed.
	// Errors are logged by the caller (ObserverGroup / txhelper); they do
	// not unwind the already-committed operation.
	ObserveAfterCommit(
		ctx context.Context,
		op string,
		resource string,
		targetEntityID *int64,
		before, after any,
	) error
}

// NoopObserver is a MutationObserver that does nothing. Useful in tests that
// need a concrete observer but do not care about its behaviour. An empty
// ObserverGroup (NewObserverGroup()) is also a valid no-op.
type NoopObserver struct{}

var _ MutationObserver = NoopObserver{}

func (NoopObserver) Observe(_ context.Context, _ pgx.Tx, _, _ string, _ *int64, _, _ any) error {
	return nil
}

func (NoopObserver) ObserveAfterCommit(_ context.Context, _, _ string, _ *int64, _, _ any) error {
	return nil
}

// Policy controls how an ObserverGroup handles in-tx observation errors
// when service methods call Observe (the default-policy variant).
//
// PolicyPropagate: the first non-nil error from any wrapped observer is
// returned, aborting the operation and rolling back the transaction.
//
// PolicySwallow: errors are logged per-observer; the group returns nil.
//
// Service methods can override the configured policy on a per-call basis by
// calling MustObserve (always propagate) or MayObserve (always swallow).
type Policy int

const (
	PolicyPropagate Policy = iota // default: fail loud
	PolicySwallow                 // best-effort: log and continue
)

// ObserverGroup wraps N MutationObserver implementations and exposes the
// three-variant in-tx call surface plus the post-commit fan-out.
//
// Service methods take *ObserverGroup (not the bare MutationObserver
// interface) so they can choose between Observe / MustObserve / MayObserve
// per call.
//
// All call variants dispatch to wrapped observers in parallel via errgroup.
// No inter-observer dependencies are supported (deliberate simplification).
// No per-observer policy split within a single group; if you need that,
// decompose into multiple groups.
//
// An empty group (NewObserverGroup() with no arguments) is valid and is a
// no-op for all methods.
type ObserverGroup struct {
	observers []MutationObserver
	policy    Policy
	logger    *slog.Logger
}

// NewObserverGroup constructs an ObserverGroup with PolicyPropagate as the
// default policy. Use WithPolicy to override.
//
// Empty input is valid and produces a no-op group; service code does not
// need nil checks.
func NewObserverGroup(observers ...MutationObserver) *ObserverGroup {
	return &ObserverGroup{observers: observers}
}

// WithPolicy returns the same ObserverGroup with the given default policy set.
// Builder-style: typically chained on the constructor.
func (g *ObserverGroup) WithPolicy(p Policy) *ObserverGroup {
	g.policy = p
	return g
}

// WithLogger attaches a logger for swallowed-error messages. If not called,
// slog.Default() is used. Useful in tests that capture log output.
func (g *ObserverGroup) WithLogger(l *slog.Logger) *ObserverGroup {
	g.logger = l
	return g
}

func (g *ObserverGroup) log() *slog.Logger {
	if g.logger != nil {
		return g.logger
	}
	return slog.Default()
}

// Observe runs all wrapped observers in parallel using the group's configured
// default policy.
func (g *ObserverGroup) Observe(
	ctx context.Context, tx pgx.Tx,
	op, resource string, targetEntityID *int64,
	before, after any,
) error {
	return g.dispatch(ctx, tx, op, resource, targetEntityID, before, after, g.policy)
}

// MustObserve runs all wrapped observers in parallel and always propagates
// the first error, regardless of the group's configured policy.
func (g *ObserverGroup) MustObserve(
	ctx context.Context, tx pgx.Tx,
	op, resource string, targetEntityID *int64,
	before, after any,
) error {
	return g.dispatch(ctx, tx, op, resource, targetEntityID, before, after, PolicyPropagate)
}

// MayObserve runs all wrapped observers in parallel and always logs and
// swallows errors, regardless of the group's configured policy.
// Always returns nil.
func (g *ObserverGroup) MayObserve(
	ctx context.Context, tx pgx.Tx,
	op, resource string, targetEntityID *int64,
	before, after any,
) error {
	return g.dispatch(ctx, tx, op, resource, targetEntityID, before, after, PolicySwallow)
}

// dispatch is the shared implementation for in-tx fan-out. It runs all
// observers in parallel; error handling is governed by policy.
func (g *ObserverGroup) dispatch(
	ctx context.Context, tx pgx.Tx,
	op, resource string, targetEntityID *int64,
	before, after any,
	policy Policy,
) error {
	if len(g.observers) == 0 {
		return nil
	}

	if policy == PolicyPropagate {
		eg, egCtx := errgroup.WithContext(ctx)
		for _, obs := range g.observers {
			obs := obs
			eg.Go(func() error {
				return obs.Observe(egCtx, tx, op, resource, targetEntityID, before, after)
			})
		}
		return eg.Wait()
	}

	// PolicySwallow: run all, log errors, return nil.
	eg, egCtx := errgroup.WithContext(ctx)
	for _, obs := range g.observers {
		obs := obs
		eg.Go(func() error {
			if err := obs.Observe(egCtx, tx, op, resource, targetEntityID, before, after); err != nil {
				g.log().ErrorContext(ctx, "observer: in-tx observe error (swallowed)",
					"op", op, "resource", resource, "error", err)
			}
			return nil
		})
	}
	_ = eg.Wait() // goroutines never return non-nil; kept for symmetry
	return nil
}

// ObserveAfterCommit runs all wrapped observers in parallel after a successful
// commit. Errors are logged per-observer; this method always returns nil.
// There are no Must/May variants because the transaction has already committed.
func (g *ObserverGroup) ObserveAfterCommit(
	ctx context.Context,
	op, resource string, targetEntityID *int64,
	before, after any,
) error {
	if len(g.observers) == 0 {
		return nil
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, obs := range g.observers {
		obs := obs
		eg.Go(func() error {
			if err := obs.ObserveAfterCommit(egCtx, op, resource, targetEntityID, before, after); err != nil {
				g.log().ErrorContext(ctx, "observer: post-commit observe error (swallowed)",
					"op", op, "resource", resource, "error", err)
			}
			return nil
		})
	}
	_ = eg.Wait()
	return nil
}
