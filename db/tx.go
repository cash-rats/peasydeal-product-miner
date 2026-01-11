package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

type TxFunc func(tx *sqlx.Tx) (any, error)

type TxFuncFormatResp func(tx *sqlx.Tx) (any, error)

func Tx(db *sqlx.DB, txFunc TxFuncFormatResp) (any, error) {
	var (
		tx  *sqlx.Tx
		err error
		res any
	)

	tx, err = db.Beginx()
	if err != nil {
		return nil, err
	}

	_, deallocErr := tx.Exec("DEALLOCATE ALL")
	if deallocErr != nil {
		_ = tx.Rollback()
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				err = fmt.Errorf("transaction rollback failed: %v (original error: %w)", rollbackErr, err)
			}
		} else {
			err = tx.Commit()
		}
	}()

	res, err = txFunc(tx)
	return res, err
}
