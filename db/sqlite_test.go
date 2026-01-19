package db

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"peasydeal-product-miner/config"
)

func TestSQLiteDisabledByDefault(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop().Sugar()

	out, err := NewSQLXSQLiteDB(NewSQLXSQLiteDBParams{
		Lc:     fxtest.NewLifecycle(t),
		Cfg:    &config.Config{},
		Logger: logger,
	})
	require.NoError(t, err)
	require.Nil(t, out.DB)

	_, err = out.Conn.Exec("select 1")
	require.ErrorIs(t, err, ErrSQLiteDisabled)
}
