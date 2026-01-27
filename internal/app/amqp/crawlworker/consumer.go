package crawlworker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"peasydeal-product-miner/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

var ErrHandlerMissing = errors.New("crawlworker handler missing")

type Handler interface {
	Handle(ctx context.Context, msg CrawlRequestedEnvelope) error
}

type Consumer struct {
	cfg     *config.Config
	channel *amqp.Channel
	handler Handler
	logger  *zap.SugaredLogger

	consumerTag string
}

type NewConsumerParams struct {
	Config  *config.Config
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
		channel:     p.Channel,
		handler:     h,
		logger:      p.Logger,
		consumerTag: "crawlworker",
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	if c.cfg == nil || strings.TrimSpace(c.cfg.RabbitMQ.URL) == "" || c.channel == nil {
		c.logger.Infow("crawlworker_disabled", "reason", "missing rabbitmq config or channel")
		return nil
	}

	if c.cfg.RabbitMQ.DeclareTopology {
		if err := c.declareTopology(ctx); err != nil {
			return err
		}
	}

	prefetch := c.cfg.RabbitMQ.Prefetch
	if prefetch <= 0 {
		prefetch = 1
	}
	if err := c.channel.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("rabbitmq qos: %w", err)
	}

	deliveries, err := c.channel.Consume(
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

	c.logger.Infow(
		"crawlworker_started",
		"queue", c.cfg.RabbitMQ.Queue,
		"prefetch", prefetch,
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				c.handleDelivery(ctx, d)
			}
		}
	}()

	return nil
}

func (c *Consumer) Stop(ctx context.Context) error {
	_ = ctx
	if c.channel == nil {
		return nil
	}
	_ = c.channel.Cancel(c.consumerTag, false)
	return nil
}

func (c *Consumer) declareTopology(ctx context.Context) error {
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

	if err := c.channel.ExchangeDeclare(ex, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq exchange declare %q: %w", ex, err)
	}
	if err := c.channel.ExchangeDeclare(dlx, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq dlx exchange declare %q: %w", dlx, err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange": dlx,
	}
	if _, err := c.channel.QueueDeclare(queueName, true, false, false, false, args); err != nil {
		return fmt.Errorf("rabbitmq queue declare %q: %w", queueName, err)
	}
	if _, err := c.channel.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return fmt.Errorf("rabbitmq dlq declare %q: %w", dlq, err)
	}

	if err := c.channel.QueueBind(queueName, routingKey, ex, false, nil); err != nil {
		return fmt.Errorf("rabbitmq queue bind queue=%q key=%q ex=%q: %w", queueName, routingKey, ex, err)
	}
	if err := c.channel.QueueBind(dlq, routingKey, dlx, false, nil); err != nil {
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
