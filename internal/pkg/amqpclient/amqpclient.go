package amqpclient

import (
	"context"
	"fmt"
	"strings"

	"peasydeal-product-miner/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type NewAMQPParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Logger    *zap.SugaredLogger
}

type AMQPOut struct {
	fx.Out

	Conn    *amqp.Connection
	Channel *amqp.Channel
}

func NewAMQP(p NewAMQPParams) (AMQPOut, error) {
	url := ""
	if p.Config != nil {
		url = strings.TrimSpace(p.Config.RabbitMQ.URL)
	}
	if url == "" {
		p.Logger.Infow("rabbitmq_disabled", "reason", "missing RABBITMQ_URL")
		return AMQPOut{Conn: nil, Channel: nil}, nil
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		return AMQPOut{}, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return AMQPOut{}, fmt.Errorf("rabbitmq channel: %w", err)
	}

	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			_ = ctx
			_ = ch.Close()
			_ = conn.Close()
			return nil
		},
	})

	if p.Config != nil {
		p.Logger.Infow(
			"rabbitmq_enabled",
			"exchange", p.Config.RabbitMQ.Exchange,
			"queue", p.Config.RabbitMQ.Queue,
			"routing_key", p.Config.RabbitMQ.RoutingKey,
			"prefetch", p.Config.RabbitMQ.Prefetch,
			"declare_topology", p.Config.RabbitMQ.DeclareTopology,
		)
	} else {
		p.Logger.Infow("rabbitmq_enabled")
	}

	return AMQPOut{Conn: conn, Channel: ch}, nil
}
