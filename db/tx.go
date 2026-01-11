package db

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type TxFunc[T any] func(*sqlx.Tx) (T, error)

func Tx[T any](ctx context.Context, db *sqlx.DB, fn TxFunc[T]) (T, error) {
	var zero T
	if db == nil {
		return zero, fmt.Errorf("db is disabled")
	}
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return zero, err
	}
	out, err := fn(tx)
	if err != nil {
		_ = tx.Rollback()
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return out, nil
}
