package dao

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"testing"
	"time"

	"peasydeal-product-miner/db"
	dbfx "peasydeal-product-miner/db/fx"
	appfx "peasydeal-product-miner/internal/app/fx"
	"peasydeal-product-miner/internal/runner"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestProductDraftStore_UpsertFromCrawlResult_E2E_TursoSQLite(t *testing.T) {
	var store *ProductDraftStore
	var conn db.Conn

	app := fx.New(
		appfx.CoreAppOptions,
		dbfx.SQLiteModule,
		fx.Provide(NewProductDraftStore),
		fx.Invoke(func(p struct {
			fx.In

			Store *ProductDraftStore
			Conn  db.Conn `name:"sqlite"`
		}) {
			store = p.Store
			conn = p.Conn
		}),
	)

	startCtx, cancelStart := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancelStart)
	require.NoError(t, app.Start(startCtx))
	t.Cleanup(func() {
		stopCtx, cancelStop := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelStop()
		_ = app.Stop(stopCtx)
	})

	var one int
	err := conn.QueryRow("select 1").Scan(&one)
	if errors.Is(err, db.ErrSQLiteDisabled) {
		t.Skip("turso sqlite is disabled; set TURSO_SQLITE_DSN/TURSO_SQLITE_PATH (+ TURSO_SQLITE_TOKEN if needed)")
	}
	require.NoError(t, err)

	var tableName string
	err = conn.QueryRow("select name from sqlite_master where type='table' and name='product_drafts'").Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing table product_drafts; run migrations (e.g. `go run ./cmd/migrate up`) against your Turso DB")
	}
	require.NoError(t, err)

	// Ensure the new column exists (migration applied).
	var hasEventIDCol bool
	rows, err := conn.Query("pragma table_info(product_drafts)")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		require.NoError(t, rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk))
		if name == "event_id" {
			hasEventIDCol = true
		}
	}
	require.NoError(t, rows.Err())
	if !hasEventIDCol {
		t.Fatalf("missing column product_drafts.event_id; run migrations (e.g. `go run ./cmd/migrate up`) against your Turso DB")
	}

	const raw = `{
  "captured_at": "2026-01-21T04:24:31.695Z",
  "currency": "TWD",
  "description": "***商品為封口袋包裝***\n\nPink Rose♥性感輕薄透膚免脫前後開檔絲襪褲襪\n材質 尼龍、彈性纖維\n顏色 多色\n產地 中國\n彈性 有\n性別 女\n件數 一件式\n內容物 褲襪*1 (不含丁字褲)\n適穿 - 胸圍 -\n適穿 - 腰圍 約 32 吋內\n適穿 - 臀圍 約 38 吋內\n適穿 - 身高 約 150 ~ 165 公分\n洗滌方式 只能手洗、不可熨燙烘乾、不可乾洗、不可漂白、陰涼吊曬\n\n#褲襪 #絲襪 #網襪 #情趣內衣 #情趣睡衣 #角色扮演 #性感睡衣 #制服 #夜店 #派對 #開檔 #透視 #性感 #免脫",
  "price": 25,
  "source": "shopee",
  "status": "ok",
  "title": "Pink Rose♥(現貨)褲襪 性感絲襪 輕薄 透膚絲襪 免脫褲襪 開檔絲襪 0151-十色任選 情趣網襪 角色扮演 | 蝦皮購物",
  "url": "https://shopee.tw/Pink-Rose%E2%99%A5(%E7%8F%BE%E8%B2%A8)%E8%A4%B2%E8%A5%AA-%E6%80%A7%E6%84%9F%E7%B5%B2%E8%A5%AA-%E8%BC%95%E8%96%84-%E9%80%8F%E8%86%9A%E7%B5%B2%E8%A5%AA-%E5%85%8D%E8%84%AB%E8%A4%B2%E8%A5%AA-%E9%96%8B%E6%AA%94%E7%B5%B2%E8%A5%AA-0151-%E5%8D%81%E8%89%B2%E4%BB%BB%E9%81%B8-%E6%83%85%E8%B6%A3%E7%B6%B2%E8%A5%AA-%E8%A7%92%E8%89%B2%E6%89%AE%E6%BC%94-i.1622185.2279887046?extraParams=%7B%22display_model_id%22%3A4404014428%2C%22model_selection_logic%22%3A3%7D\u0026sp_atk=99eb87ef-bdd1-4a1d-bdf1-d55e2a7f850c\u0026xptdk=99eb87ef-bdd1-4a1d-bdf1-d55e2a7f850c"
}`

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	res := runner.Result(m)

	url, _ := m["url"].(string)
	require.NotEmpty(t, url)

	eventID := uuid.NewString()
	draftID, err := store.UpsertFromCrawlResult(context.Background(), UpsertFromCrawlResultInput{
		EventID:   eventID,
		CreatedBy: "test",
		URL:       url,
		Result:    res,
	})
	require.NoError(t, err)
	require.NotEmpty(t, draftID)

	t.Cleanup(func() {
		_, _ = conn.Exec(conn.Rebind("DELETE FROM product_drafts WHERE id = ?"), draftID)
	})

	var gotPayload string
	var gotStatus string
	var gotError sql.NullString
	var gotURL string
	var gotEventID sql.NullString
	require.NoError(t, conn.QueryRow(
		conn.Rebind("SELECT draft_payload, status, error, url, event_id FROM product_drafts WHERE id = ?"),
		draftID,
	).Scan(&gotPayload, &gotStatus, &gotError, &gotURL, &gotEventID))

	log.Printf("~~ %v", gotPayload)

	require.True(t, gotEventID.Valid)
	require.Equal(t, eventID, gotEventID.String)

	// require.Equal(t, "READY_FOR_REVIEW", gotStatus)
	// require.False(t, gotError.Valid)
	// require.Equal(t, url, gotURL)
	//
	// var got map[string]any
	// require.NoError(t, json.Unmarshal([]byte(gotPayload), &got))
	// require.Equal(t, url, got["url"])
	// require.Equal(t, "ok", got["status"])
	// require.Equal(t, "shopee", got["source"])
	// require.Equal(t, "TWD", got["currency"])
	// require.Equal(t, "2026-01-21T04:24:31.695Z", got["captured_at"])
	// require.Equal(t, "Pink Rose♥(現貨)褲襪 性感絲襪 輕薄 透膚絲襪 免脫褲襪 開檔絲襪 0151-十色任選 情趣網襪 角色扮演 | 蝦皮購物", got["title"])
}
