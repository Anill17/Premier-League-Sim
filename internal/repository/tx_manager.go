// Package repository contains the PostgreSQL-backed implementations of
// the repository and TxManager interfaces declared in internal/domain.
// Repositories are dumb in the strictest sense: they execute SQL and
// nothing else. All business rules live in internal/service.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anill17/league-api/internal/domain"
)

// ErrNotFound is the sentinel returned by every Get-style repository
// method when a row does not exist. Services map this to the
// appropriate HTTP 404 code via pkg/response error constants.
var ErrNotFound = errors.New("repository: not found")

// PgxTxManager is the production TxManager backed by pgxpool.Pool.
// It guarantees two things about every fn it runs:
//
//  1. The transaction is opened with the requested isolation level
//     before fn runs.
//  2. The transaction is rolled back if fn returns an error OR panics;
//     it is committed only on the happy path.
type PgxTxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager wires a pgxpool.Pool into a domain.TxManager.
func NewTxManager(pool *pgxpool.Pool) *PgxTxManager {
	return &PgxTxManager{pool: pool}
}

// Compile-time guarantee that PgxTxManager satisfies the interface.
var _ domain.TxManager = (*PgxTxManager)(nil)

// WithTransaction runs fn in a READ COMMITTED transaction.
func (m *PgxTxManager) WithTransaction(
	ctx context.Context,
	fn func(tx pgx.Tx) error,
) error {
	return m.run(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

// WithSerializableTransaction runs fn in a SERIALIZABLE transaction.
// SERIALIZABLE may produce 40001 (serialization_failure) errors; the
// caller is responsible for retry policy.
func (m *PgxTxManager) WithSerializableTransaction(
	ctx context.Context,
	fn func(tx pgx.Tx) error,
) error {
	return m.run(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable}, fn)
}

func (m *PgxTxManager) run(
	ctx context.Context,
	opts pgx.TxOptions,
	fn func(tx pgx.Tx) error,
) (err error) {
	tx, err := m.pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
			}
			return
		}
		if cErr := tx.Commit(ctx); cErr != nil {
			err = fmt.Errorf("commit tx: %w", cErr)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	return nil
}
