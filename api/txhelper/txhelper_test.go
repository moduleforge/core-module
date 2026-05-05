package txhelper_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/moduleforge/core-api/observer"
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

// -- counting observer --

type countObserver struct {
	afterCommitCalls atomic.Int32
	afterCommitErr   error
}

func (o *countObserver) Observe(_ context.Context, _ pgx.Tx, _, _ string, _ *int64, _, _ any) error {
	return nil
}

func (o *countObserver) ObserveAfterCommit(_ context.Context, _, _ string, _ *int64, _, _ any) error {
	o.afterCommitCalls.Add(1)
	return o.afterCommitErr
}

// -- helpers --

func newLogBuf() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
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

func TestRun_PostCommitFiresOnSuccess(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}
	obs := &countObserver{}

	err := txhelper.Run(context.Background(), db, func(ctx context.Context, _ pgx.Tx) error {
		txhelper.QueuePostCommit(ctx, obs, "create", "foo", nil, nil, nil)
		return nil
	})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if got := obs.afterCommitCalls.Load(); got != 1 {
		t.Fatalf("expected 1 post-commit call, got %d", got)
	}
}

func TestRun_PostCommitDoesNotFireOnFnError(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}
	obs := &countObserver{}
	fnErr := errors.New("fn failed")

	_ = txhelper.Run(context.Background(), db, func(ctx context.Context, _ pgx.Tx) error {
		txhelper.QueuePostCommit(ctx, obs, "create", "foo", nil, nil, nil)
		return fnErr
	})

	if got := obs.afterCommitCalls.Load(); got != 0 {
		t.Fatalf("expected 0 post-commit calls after rollback, got %d", got)
	}
}

func TestRun_PostCommitErrorLogged_NotPropagated(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}
	logger, buf := newLogBuf()

	obs := &countObserver{afterCommitErr: errors.New("post-commit failure")}

	err := txhelper.RunWithLogger(context.Background(), db, func(ctx context.Context, _ pgx.Tx) error {
		txhelper.QueuePostCommit(ctx, obs, "create", "foo", nil, nil, nil)
		return nil
	}, logger)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("post-commit failure")) {
		t.Errorf("expected post-commit error in log; got: %s", buf.String())
	}
}

func TestQueuePostCommit_PanicsOutsideRun(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when QueuePostCommit called outside Run ctx")
		}
	}()
	// Plain background context — no queue attached.
	txhelper.QueuePostCommit(context.Background(), observer.NoopObserver{}, "create", "foo", nil, nil, nil)
}

func TestRun_MultiplePostCommitObservers(t *testing.T) {
	tx := &fakeTx{}
	db := &fakeDB{tx: tx}

	obs1 := &countObserver{}
	obs2 := &countObserver{}

	err := txhelper.Run(context.Background(), db, func(ctx context.Context, _ pgx.Tx) error {
		txhelper.QueuePostCommit(ctx, obs1, "create", "foo", nil, nil, nil)
		txhelper.QueuePostCommit(ctx, obs2, "create", "bar", nil, nil, nil)
		return nil
	})
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if got := obs1.afterCommitCalls.Load(); got != 1 {
		t.Errorf("obs1: expected 1 post-commit call, got %d", got)
	}
	if got := obs2.afterCommitCalls.Load(); got != 1 {
		t.Errorf("obs2: expected 1 post-commit call, got %d", got)
	}
}
