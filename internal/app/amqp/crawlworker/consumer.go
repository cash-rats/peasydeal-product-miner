package crawlworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"strings"
	"time"

	"peasydeal-product-miner/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var ErrHandlerMissing = errors.New("crawlworker handler missing")

type Handler interface {
	Handle(ctx context.Context, msg CrawlRequestedEnvelope) error
}

type Consumer struct {
	cfg     *config.Config
	conn    *amqp.Connection
	channel *amqp.Channel
	handler Handler
	logger  *zap.SugaredLogger

	consumerTag string
	consumeCtx  context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
}

type NewConsumerParams struct {
	fx.In

	Config  *config.Config
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Handler Handler `optional:"true"`
	Logger  *zap.SugaredLogger
}

func NewConsumer(p NewConsumerParams) *Consumer {
	h := p.Handler
	if h == nil {
		h = missingHandler{}
	}

	return &Consumer{
		cfg:         p.Config,
		conn:        p.Conn,
		channel:     p.Channel,
		handler:     h,
		logger:      p.Logger,
		consumerTag: "crawlworker",
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	if c.cfg == nil || strings.TrimSpace(c.cfg.RabbitMQ.URL) == "" {
		c.logger.Infow("crawlworker_disabled", "reason", "missing rabbitmq config")
		return nil
	}

	if c.consumeCtx == nil || c.cancel == nil {
		c.consumeCtx, c.cancel = context.WithCancel(context.Background())
	}

	go c.runConsumeLoop(c.consumeCtx)

	return nil
}

func (c *Consumer) runConsumeLoop(ctx context.Context) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		if err := c.consumeOnce(ctx); err != nil {
			c.logger.Warnw("crawlworker_consume_cycle_failed", "err", err)
		}

		if ctx.Err() != nil {
			return
		}

		c.logger.Infow("crawlworker_reconnecting", "backoff", backoff.String())
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (c *Consumer) Stop(ctx context.Context) error {
	_ = ctx
	if c.cancel != nil {
		c.cancel()
	}
	c.closeCurrent()
	return nil
}

func (c *Consumer) declareTopology(ctx context.Context, ch *amqp.Channel) error {
	_ = ctx

	ex := strings.TrimSpace(c.cfg.RabbitMQ.Exchange)
	if ex == "" {
		ex = "events"
	}

	queueName := strings.TrimSpace(c.cfg.RabbitMQ.Queue)
	if queueName == "" {
		queueName = "crawler.url.requested.v1"
	}

	routingKey := strings.TrimSpace(c.cfg.RabbitMQ.RoutingKey)
	if routingKey == "" {
		routingKey = "crawler.url.requested.v1"
	}

	dlx := ex + ".dlx"
	dlq := queueName + ".dlq"

	if err := ch.ExchangeDeclare(ex, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq exchange declare %q: %w", ex, err)
	}
	if err := ch.ExchangeDeclare(dlx, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq dlx exchange declare %q: %w", dlx, err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange": dlx,
	}
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, args); err != nil {
		return fmt.Errorf("rabbitmq queue declare %q: %w", queueName, err)
	}
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq dlq declare %q: %w", dlq, err)
	}

	if err := ch.QueueBind(queueName, routingKey, ex, false, nil); err != nil {
		return fmt.Errorf("rabbitmq queue bind queue=%q key=%q ex=%q: %w", queueName, routingKey, ex, err)
	}
	if err := ch.QueueBind(dlq, routingKey, dlx, false, nil); err != nil {
		return fmt.Errorf("rabbitmq dlq bind queue=%q key=%q ex=%q: %w", dlq, routingKey, dlx, err)
	}

	c.logger.Infow(
		"crawlworker_topology_declared",
		"exchange", ex,
		"queue", queueName,
		"routing_key", routingKey,
		"dlx", dlx,
		"dlq", dlq,
	)

	return nil
}

func (c *Consumer) consumeOnce(ctx context.Context) error {
	conn, ch, err := c.ensureChannel()
	if err != nil {
		return err
	}

	if c.cfg.RabbitMQ.DeclareTopology {
		if err := c.declareTopology(ctx, ch); err != nil {
			return err
		}
	}

	prefetch := c.cfg.RabbitMQ.Prefetch
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("rabbitmq qos: %w", err)
	}

	deliveries, err := ch.Consume(
		c.cfg.RabbitMQ.Queue,
		c.consumerTag,
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("rabbitmq consume: %w", err)
	}

	connClose := conn.NotifyClose(make(chan *amqp.Error, 1))
	chClose := ch.NotifyClose(make(chan *amqp.Error, 1))
	chCancel := ch.NotifyCancel(make(chan string, 1))

	c.logger.Infow(
		"crawlworker_started",
		"queue", c.cfg.RabbitMQ.Queue,
		"prefetch", prefetch,
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-connClose:
			if err != nil {
				c.logger.Warnw("crawlworker_connection_closed", "err", err)
			} else {
				c.logger.Warnw("crawlworker_connection_closed")
			}
			return fmt.Errorf("rabbitmq connection closed")
		case err := <-chClose:
			if err != nil {
				c.logger.Warnw("crawlworker_channel_closed", "err", err)
			} else {
				c.logger.Warnw("crawlworker_channel_closed")
			}
			return fmt.Errorf("rabbitmq channel closed")
		case reason := <-chCancel:
			c.logger.Warnw("crawlworker_channel_cancelled", "reason", reason)
			return fmt.Errorf("rabbitmq channel cancelled")
		case d, ok := <-deliveries:
			if !ok {
				c.logger.Warnw("crawlworker_deliveries_closed")
				return fmt.Errorf("rabbitmq deliveries closed")
			}
			c.handleDelivery(ctx, d)
		}
	}
}

func (c *Consumer) ensureChannel() (*amqp.Connection, *amqp.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosed() && c.channel != nil && !c.channel.IsClosed() {
		return c.conn, c.channel, nil
	}

	if c.channel != nil {
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}

	conn, ch, err := c.dial()
	if err != nil {
		return nil, nil, err
	}
	c.conn = conn
	c.channel = ch
	return conn, ch, nil
}

func (c *Consumer) closeCurrent() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		_ = c.channel.Cancel(c.consumerTag, false)
		_ = c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *Consumer) dial() (*amqp.Connection, *amqp.Channel, error) {
	url := ""
	if c.cfg != nil {
		url = strings.TrimSpace(c.cfg.RabbitMQ.URL)
	}
	if url == "" {
		return nil, nil, fmt.Errorf("rabbitmq url missing")
	}

	conn, err := amqp.DialConfig(url, amqp.Config{
		Heartbeat: 10 * time.Second,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	return conn, ch, nil
}

func (c *Consumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	eventID := strings.TrimSpace(d.MessageId)
	if eventID == "" {
		eventID = strings.TrimSpace(d.CorrelationId)
	}

	var msg CrawlRequestedEnvelope
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		c.logger.Errorw("crawlworker_invalid_json",
			"err", err,
			"message_id", eventID,
		)
		_ = d.Reject(false)
		return
	}

	if strings.TrimSpace(msg.EventID) == "" && eventID != "" {
		msg.EventID = eventID
	}

	if strings.TrimSpace(msg.EventID) == "" {
		c.logger.Errorw("crawlworker_missing_event_id",
			"message_id", eventID,
			"event_name", msg.EventName,
		)
		_ = d.Reject(false)
		return
	}

	if err := c.handler.Handle(ctx, msg); err != nil {
		c.logger.Errorw("crawlworker_handle_failed",
			"err", err,
			"event_id", msg.EventID,
			"event_name", msg.EventName,
		)
		_ = d.Reject(false)
		return
	}

	_ = d.Ack(false)
}

type missingHandler struct{}

func (missingHandler) Handle(ctx context.Context, msg CrawlRequestedEnvelope) error {
	_ = ctx
	_ = msg
	return ErrHandlerMissing
}
