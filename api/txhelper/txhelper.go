// Package txhelper owns transaction lifecycle for service methods. It is a
// thin helper; business logic lives in service methods.
//
// # Logger injection
//
// RunWithLogger accepts a *slog.Logger. Run uses slog.Default(). Pass a custom
// logger in tests to capture log output.
package txhelper

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// DB is the minimal interface txhelper needs from a connection pool.
// pgxpool.Pool satisfies it.
type DB interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// Run executes fn inside a transaction. On fn success, the tx is committed.
// On fn error, the tx is rolled back.
func Run(
	ctx context.Context,
	db DB,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	return RunWithLogger(ctx, db, fn, nil)
}

// RunSerializable is like Run but opens the transaction at the Serializable
// isolation level. Use for operations where a TOCTOU race between a read and
// a write could cause data-integrity violations (e.g. last-identity safety
// checks in Unlink and RemovePassword).
func RunSerializable(
	ctx context.Context,
	db DB,
	fn func(ctx context.Context, tx pgx.Tx) error,
) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("txhelper: begin tx: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("txhelper: commit: %w", err)
	}

	return nil
}

// RunWithLogger is like Run but accepts a *slog.Logger. When logger is nil,
// slog.Default() is used.
func RunWithLogger(
	ctx context.Context,
	db DB,
	fn func(ctx context.Context, tx pgx.Tx) error,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	_ = logger // retained for future use; no post-commit dispatch here

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("txhelper: begin tx: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		// Roll back on any fn error; swallow rollback errors to preserve the
		// original error returned to the caller.
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("txhelper: commit: %w", err)
	}

	return nil
}
