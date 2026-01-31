package enqueue

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/app/amqp/crawlworker"
	"peasydeal-product-miner/internal/app/amqp/productdrafts"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func TestHandler_Handle_BadJSON(t *testing.T) {
	h := &Handler{cfg: &config.Config{}, logger: zap.NewNop().Sugar()}

	req := httptest.NewRequest(http.MethodPost, "/v1/crawl/enqueue", strings.NewReader("{"))
	w := httptest.NewRecorder()

	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_Handle_MissingURL(t *testing.T) {
	h := &Handler{cfg: &config.Config{}, logger: zap.NewNop().Sugar()}

	req := httptest.NewRequest(http.MethodPost, "/v1/crawl/enqueue", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_Handle_UnsupportedDomain(t *testing.T) {
	h := &Handler{cfg: &config.Config{}, logger: zap.NewNop().Sugar()}

	req := httptest.NewRequest(http.MethodPost, "/v1/crawl/enqueue", strings.NewReader(`{"url":"https://example.com/x"}`))
	w := httptest.NewRecorder()

	h.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_Handle_RabbitMQDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.RabbitMQ.URL = ""
	h := &Handler{cfg: cfg, logger: zap.NewNop().Sugar()}

	req := httptest.NewRequest(http.MethodPost, "/v1/crawl/enqueue", strings.NewReader(`{"url":"https://shopee.tw/p/1"}`))
	w := httptest.NewRecorder()

	h.Handle(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestHandler_Handle_OK_PublishesDeterministicEventID(t *testing.T) {
	var gotExchange, gotKey string
	var gotPublishing amqp.Publishing
	var gotQueuedEventID, gotQueuedURL, gotQueuedSource string
	var gotResp struct {
		OK      bool   `json:"ok"`
		EventID string `json:"event_id"`
		ID      string `json:"id"`
	}

	cfg := &config.Config{}
	cfg.RabbitMQ.URL = "amqp://example"
	cfg.RabbitMQ.Exchange = "events"
	cfg.RabbitMQ.RoutingKey = "crawler.url.requested.v1"
	cfg.RabbitMQ.DeclareTopology = false

	h := &Handler{
		cfg:           cfg,
		logger:        zap.NewNop().Sugar(),
		sqliteEnabled: true,
		store: queuedDraftWriterFunc(func(ctx context.Context, in productdrafts.UpsertQueuedForDraftInput) (string, error) {
			_ = ctx
			gotQueuedEventID = in.EventID
			gotQueuedURL = in.URL
			gotQueuedSource = in.Source
			return "draft-1", nil
		}),
		publish: func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
			_ = ctx
			_ = mandatory
			_ = immediate
			gotExchange = exchange
			gotKey = key
			gotPublishing = msg
			return nil
		},
	}

	url := "https://shopee.tw/p/1"
	req := httptest.NewRequest(http.MethodPost, "/v1/crawl/enqueue", strings.NewReader(`{"url":"`+url+`"}`))
	w := httptest.NewRecorder()

	before := time.Now().UTC().Add(-1 * time.Second)
	h.Handle(w, req)
	after := time.Now().UTC().Add(1 * time.Second)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &gotResp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, w.Body.String())
	}
	if !gotResp.OK {
		t.Fatalf("response ok=false body=%s", w.Body.String())
	}
	if gotResp.EventID != eventIDFromURL(url) {
		t.Fatalf("response event_id=%q expected=%q", gotResp.EventID, eventIDFromURL(url))
	}
	if gotResp.ID != "draft-1" {
		t.Fatalf("response id=%q expected=%q", gotResp.ID, "draft-1")
	}
	if gotExchange != "events" || gotKey != "crawler.url.requested.v1" {
		t.Fatalf("publish exchange=%q key=%q", gotExchange, gotKey)
	}
	if gotPublishing.ContentType != "application/json" {
		t.Fatalf("contentType=%q", gotPublishing.ContentType)
	}
	if gotPublishing.MessageId == "" {
		t.Fatalf("missing message id")
	}
	if gotPublishing.MessageId != eventIDFromURL(url) {
		t.Fatalf("event_id=%q expected=%q", gotPublishing.MessageId, eventIDFromURL(url))
	}
	if gotQueuedEventID != eventIDFromURL(url) {
		t.Fatalf("queued event_id=%q expected=%q", gotQueuedEventID, eventIDFromURL(url))
	}
	if gotQueuedURL != url {
		t.Fatalf("queued url=%q expected=%q", gotQueuedURL, url)
	}
	if gotQueuedSource != "shopee" {
		t.Fatalf("queued source=%q expected=%q", gotQueuedSource, "shopee")
	}
	if gotPublishing.Timestamp.Before(before) || gotPublishing.Timestamp.After(after) {
		t.Fatalf("timestamp=%s out of range", gotPublishing.Timestamp)
	}

	var env crawlworker.CrawlRequestedEnvelope
	if err := json.Unmarshal(gotPublishing.Body, &env); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if env.EventID != eventIDFromURL(url) {
		t.Fatalf("env.event_id=%q expected=%q", env.EventID, eventIDFromURL(url))
	}
	if env.EventName != "crawler/url.requested" {
		t.Fatalf("env.event_name=%q", env.EventName)
	}
	if env.Data.URL != url {
		t.Fatalf("env.data.url=%q", env.Data.URL)
	}
	if env.Data.OutDir != "" {
		t.Fatalf("env.data.out_dir should be empty, got %q", env.Data.OutDir)
	}
}

type queuedDraftWriterFunc func(ctx context.Context, in productdrafts.UpsertQueuedForDraftInput) (string, error)

func (f queuedDraftWriterFunc) UpsertQueuedForDraft(ctx context.Context, in productdrafts.UpsertQueuedForDraftInput) (string, error) {
	return f(ctx, in)
}
