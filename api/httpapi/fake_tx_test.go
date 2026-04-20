package httpapi

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeTx is a minimal pgx.Tx that does nothing — suitable for handler tests
// that use fake services (which never touch the DB).
type fakeTx struct {
	commitErr   error
	rollbackErr error
}

func (f *fakeTx) Begin(_ context.Context) (pgx.Tx, error) { return f, nil }
func (f *fakeTx) Commit(_ context.Context) error          { return f.commitErr }
func (f *fakeTx) Rollback(_ context.Context) error        { return f.rollbackErr }
func (f *fakeTx) CopyFrom(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("not implemented")
}
func (f *fakeTx) SendBatch(_ context.Context, _ *pgx.Batch) pgx.BatchResults {
	return nil
}
func (f *fakeTx) LargeObjects() pgx.LargeObjects { return pgx.LargeObjects{} }
func (f *fakeTx) Prepare(_ context.Context, _, _ string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("not implemented")
}
func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return nil }
func (f *fakeTx) Conn() *pgx.Conn                                        { return nil }

// fakeTxBeginner implements TxBeginner using a fakeTx.
type fakeTxBeginner struct {
	tx  *fakeTx
	err error
}

func (f *fakeTxBeginner) Begin(_ context.Context) (pgx.Tx, error) {
	return f.tx, f.err
}
