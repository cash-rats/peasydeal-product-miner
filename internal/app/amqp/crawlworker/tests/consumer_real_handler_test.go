package tests

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/app/amqp/crawlworker"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CrawlWorkerRealHandlerSuite struct {
	suite.Suite

	cfg *config.Config
	cwd string
}

func TestCrawlWorkerRealHandlerSuite(t *testing.T) {
	suite.Run(t, new(CrawlWorkerRealHandlerSuite))
}

func (s *CrawlWorkerRealHandlerSuite) SetupTest() {
	if strings.TrimSpace(os.Getenv("RABBITMQ_URL")) == "" {
		s.T().Skip("RABBITMQ_URL is required for integration test")
	}

	cwd, err := os.Getwd()
	require.NoError(s.T(), err)
	s.cwd = cwd

	repoRoot, err := findRepoRoot(cwd)
	require.NoError(s.T(), err)
	require.NoError(s.T(), os.Chdir(repoRoot))
	require.NoError(s.T(), os.MkdirAll("out", 0o755))

	vp := config.NewViper()
	cfg, err := config.NewConfig(vp)
	require.NoError(s.T(), err)
	s.cfg = cfg
}

func (s *CrawlWorkerRealHandlerSuite) TearDownTest() {
	if s.cwd != "" {
		_ = os.Chdir(s.cwd)
	}
}

func (s *CrawlWorkerRealHandlerSuite) TestConsumeAndPersistRealHandler() {
	rabbitURL := strings.TrimSpace(os.Getenv("RABBITMQ_URL"))
	require.NotEmpty(s.T(), rabbitURL)

	conn, err := amqp.Dial(rabbitURL)
	require.NoError(s.T(), err)
	ch, err := conn.Channel()
	require.NoError(s.T(), err)
	defer func() {
		_ = ch.Close()
		_ = conn.Close()
	}()

	exchange := strings.TrimSpace(s.cfg.RabbitMQ.Exchange)
	routingKey := strings.TrimSpace(s.cfg.RabbitMQ.RoutingKey)
	require.NotEmpty(s.T(), exchange)
	require.NotEmpty(s.T(), routingKey)

	require.NoError(s.T(), ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil))

	eventID := "amqp-real-" + time.Now().UTC().Format("20060102T150405.000000000Z07:00")

	msg := crawlworker.CrawlRequestedEnvelope{
		EventName: "crawler/url.requested",
		EventID:   eventID,
		TS:        time.Now().UTC(),
		Data: crawlworker.CrawlRequestedEventData{
			URL:    "https://shopee.tw/%E3%80%90%E5%B2%B1%E6%80%9D%E5%A4%A2%E3%80%91%E6%A5%B5%E6%9F%94%E5%A4%A9%E7%B5%B2%E5%BA%8A%E5%8C%85%E7%B5%84-%E5%96%AE%E4%BA%BA-%E9%9B%99%E4%BA%BA-%E5%8A%A0%E5%A4%A7-%E7%89%B9%E5%A4%A7-%E5%85%A9%E7%94%A8%E8%A2%AB%E5%BA%8A%E5%8C%85%E7%B5%84-%E5%8A%A0%E9%AB%9835%E5%85%AC%E5%88%86-%E8%90%8A%E8%B3%BD%E7%88%BE%E7%BA%96%E7%B6%AD-3M%E5%90%B8%E6%BF%95%E6%8E%92%E6%B1%97-M1-i.3746101.513091408",
			OutDir: "out",
		},
	}

	if msg.Data.URL == "" {
		s.T().Skip("set AMQP_E2E_URL to a real product URL for the real handler integration test")
	}

	body, err := json.Marshal(msg)
	require.NoError(s.T(), err)

	err = ch.PublishWithContext(
		context.Background(),
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    eventID,
			Type:         "crawler/url.requested",
			Body:         body,
		},
	)
	require.NoError(s.T(), err)

	_ = eventID
}

func findRepoRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
