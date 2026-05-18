package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/insider/league-api/internal/domain"
)

// TxManagerMock is a domain.TxManager that, by default, executes the
// callback inline with a nil pgx.Tx. Mock repositories ignore the tx
// argument so the nil is harmless.
type TxManagerMock struct {
	WithTransactionFunc             func(ctx context.Context, fn func(tx pgx.Tx) error) error
	WithSerializableTransactionFunc func(ctx context.Context, fn func(tx pgx.Tx) error) error

	WithTransactionCalls             int
	WithSerializableTransactionCalls int
}

var _ domain.TxManager = (*TxManagerMock)(nil)

func (m *TxManagerMock) WithTransaction(
	ctx context.Context,
	fn func(tx pgx.Tx) error,
) error {
	m.WithTransactionCalls++
	if m.WithTransactionFunc != nil {
		return m.WithTransactionFunc(ctx, fn)
	}
	return fn(nil)
}

func (m *TxManagerMock) WithSerializableTransaction(
	ctx context.Context,
	fn func(tx pgx.Tx) error,
) error {
	m.WithSerializableTransactionCalls++
	if m.WithSerializableTransactionFunc != nil {
		return m.WithSerializableTransactionFunc(ctx, fn)
	}
	return fn(nil)
}
