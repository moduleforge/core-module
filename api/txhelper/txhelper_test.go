package txhelper_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/moduleforge/core-api/txhelper"
)

// -- fake pgx.Tx --

// fakeTx implements pgx.Tx with controllable commit/rollback outcomes and
// records which operations were called.
type fakeTx struct {
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

func (t *fakeTx) Begin(ctx context.Context) (pgx.Tx, error) {
	return nil, errors.New("fakeTx.Begin not implemented")
}

func (t *fakeTx) Commit(ctx context.Context) error {
	t.committed = true
	return t.commitErr
}

func (t *fakeTx) Rollback(ctx context.Context) error {
	t.rolledBack = true
	return t.rollbackErr
}

func (t *fakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("fakeTx.Exec not implemented")
}

func (t *fakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, errors.New("fakeTx.Query not implemented")
}

func (t *fakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (t *fakeTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("fakeTx.CopyFrom not implemented")
}

func (t *fakeTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	return nil
}

func (t *fakeTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (t *fakeTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("fakeTx.Prepare not implemented")
}

func (t *fakeTx) Conn() *pgx.Conn {
	return nil
}

// -- fake DB --

// fakeDB implements txhelper.DB. It returns the configured tx on BeginTx.
type fakeDB struct {
	tx  pgx.Tx
	err error
}

func (d *fakeDB) BeginTx(_ context.Context, _ pgx.TxOptions) (pgx.Tx, error) {
	return d.tx, d.err
}

// -- tests --

func TestRun_CommitsOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}

	err := txhelper.Run(context.Background(), db, func(ctx context.Context, tx pgx.Tx) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if !tx.committed {
		t.Fatal("expected tx to be committed")
	}
	if tx.rolledBack {
		t.Fatal("expected tx NOT to be rolled back")
	}
}

func TestRun_RollsBackOnFnError(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}
	sentinel := errors.New("fn error")

	err := txhelper.Run(context.Background(), db, func(ctx context.Context, tx pgx.Tx) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Run: expected sentinel error, got: %v", err)
	}
	if tx.committed {
		t.Fatal("expected tx NOT to be committed")
	}
	if !tx.rolledBack {
		t.Fatal("expected tx to be rolled back")
	}
}

func TestRun_PropagatesFnError(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}
	sentinel := errors.New("propagated error")

	err := txhelper.Run(context.Background(), db, func(ctx context.Context, tx pgx.Tx) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected fn error to be propagated verbatim; got: %v", err)
	}
}
