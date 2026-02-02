package enqueue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/app/amqp/crawlworker"
	"peasydeal-product-miner/internal/app/amqp/productdrafts"
	"peasydeal-product-miner/internal/pkg/render"
	"peasydeal-product-miner/internal/router"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Handler struct {
	cfg           *config.Config
	channel       *amqp.Channel
	logger        *zap.SugaredLogger
	store         queuedDraftWriter
	sqliteEnabled bool

	publish func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

type queuedDraftWriter interface {
	UpsertQueuedForDraft(ctx context.Context, in productdrafts.UpsertQueuedForDraftInput) (draftID string, err error)
}

type NewHandlerParams struct {
	fx.In

	Cfg      *config.Config
	Channel  *amqp.Channel `optional:"true"`
	Logger   *zap.SugaredLogger
	Store    *productdrafts.ProductDraftStore `optional:"true"`
	SQLiteDB *sqlx.DB                         `name:"sqlite" optional:"true"`
}

func NewHandler(p NewHandlerParams) *Handler {
	var publishFn func(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	if p.Channel != nil {
		publishFn = p.Channel.PublishWithContext
	}

	return &Handler{
		cfg:           p.Cfg,
		channel:       p.Channel,
		logger:        p.Logger,
		store:         p.Store,
		sqliteEnabled: p.SQLiteDB != nil,
		publish:       publishFn,
	}
}

func (h *Handler) RegisterRoute(r *chi.Mux) {
	r.Post("/v1/crawl/enqueue", h.Handle)
}

type enqueueRequest struct {
	URL string `json:"url"`
}

type enqueueResponse struct {
	OK      bool   `json:"ok"`
	EventID string `json:"event_id"`
	ID      string `json:"id,omitempty"`
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var req enqueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.ChiErr(w, http.StatusBadRequest, "invalid json")
		return
	}

	rawURL := req.URL
	if rawURL == "" {
		render.ChiErr(w, http.StatusBadRequest, "missing url")
		return
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		render.ChiErr(w, http.StatusBadRequest, "invalid url")
		return
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		render.ChiErr(w, http.StatusBadRequest, "url must be http(s)")
		return
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		render.ChiErr(w, http.StatusBadRequest, "invalid url host")
		return
	}
	if !isSupportedCommerceHost(host) {
		render.ChiErr(w, http.StatusBadRequest, "unsupported url domain (supported: shopee, taobao)")
		return
	}

	log.Printf("~~ ** 1 %v", h.cfg.RabbitMQ.URL)

	if h.cfg.RabbitMQ.URL == "" || h.publish == nil {
		render.ChiErr(w, http.StatusServiceUnavailable, "rabbitmq disabled")
		return
	}

	ex := h.cfg.RabbitMQ.Exchange
	if ex == "" {
		ex = "events"
	}
	routingKey := h.cfg.RabbitMQ.RoutingKey
	if routingKey == "" {
		routingKey = "crawler.url.requested.v1"
	}

	now := time.Now().UTC()
	eventID := eventIDFromURL(rawURL)
	draftID := ""

	if h.store != nil && h.sqliteEnabled {
		source := sourceFromHost(host)
		id, err := h.store.UpsertQueuedForDraft(r.Context(), productdrafts.UpsertQueuedForDraftInput{
			EventID:   eventID,
			CreatedBy: "enqueue",
			URL:       rawURL,
			Source:    source,
		})
		if err != nil {
			h.logger.Errorw("enqueue_persist_queued_failed", "event_id", eventID, "url", rawURL, "err", err)
		} else {
			draftID = id
		}
	}

	env := crawlworker.CrawlRequestedEnvelope{
		EventName: "crawler/url.requested",
		EventID:   eventID,
		TS:        now,
		Data: crawlworker.CrawlRequestedEventData{
			URL: rawURL,
		},
	}
	body, err := json.Marshal(env)
	if err != nil {
		h.logger.Errorw("enqueue_marshal_failed", "err", err)
		render.ChiErr(w, http.StatusInternalServerError, "failed to encode message")
		return
	}

	if h.channel != nil && h.cfg.RabbitMQ.DeclareTopology {
		if err := h.channel.ExchangeDeclare(ex, "topic", true, false, false, false, nil); err != nil {
			h.logger.Errorw("enqueue_exchange_declare_failed", "exchange", ex, "err", err)
			render.ChiErr(w, http.StatusBadGateway, fmt.Sprintf("rabbitmq exchange declare failed: %s", ex))
			return
		}
	}

	if err := h.publish(r.Context(), ex, routingKey, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Timestamp:    now,
		MessageId:    eventID,
		Body:         body,
	}); err != nil {
		h.logger.Errorw(
			"enqueue_publish_failed",
			"exchange", ex,
			"routing_key", routingKey,
			"event_id", eventID,
			"url", rawURL,
			"err", err,
		)
		render.ChiErr(w, http.StatusBadGateway, "failed to publish message")
		return
	}

	h.logger.Infow("enqueue_published", "exchange", ex, "routing_key", routingKey, "event_id", eventID, "url", rawURL)
	render.ChiJSON(w, http.StatusOK, enqueueResponse{OK: true, EventID: eventID, ID: draftID})
}

func eventIDFromURL(u string) string {
	sum := sha256.Sum256([]byte(u))
	return "urlsha256:" + hex.EncodeToString(sum[:])
}

func isSupportedCommerceHost(host string) bool {
	if host == "taobao.com" || strings.HasSuffix(host, ".taobao.com") {
		return true
	}
	if host == "shopee.com" || strings.HasSuffix(host, ".shopee.com") {
		return true
	}
	// Regional Shopee domains: shopee.<tld> and *.shopee.<tld>
	if strings.HasPrefix(host, "shopee.") || strings.Contains(host, ".shopee.") {
		return true
	}
	return false
}

func sourceFromHost(host string) string {
	if host == "taobao.com" || strings.HasSuffix(host, ".taobao.com") {
		return "taobao"
	}
	// Shopee includes "shopee.com", regional "shopee.<tld>", and subdomains.
	if strings.HasPrefix(host, "shopee.") || strings.Contains(host, ".shopee.") || host == "shopee.com" || strings.HasSuffix(host, ".shopee.com") {
		return "shopee"
	}
	return ""
}

var _ router.Handler = (*Handler)(nil)
