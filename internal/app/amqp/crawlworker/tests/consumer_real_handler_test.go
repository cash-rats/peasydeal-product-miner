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
			URL:    "https://shopee.tw/OOFY%E9%9D%B4-%E5%A4%A7%E5%B0%BA%E7%A2%BC-%E8%8B%B1%E5%80%AB%E5%BE%A9%E5%8F%A4%E7%99%BD%E8%89%B2%E8%BB%8A%E7%B8%AB%E7%B7%9A%E5%8E%9A%E5%BA%95%E9%A6%AC%E4%B8%81%E7%9A%AE%E9%9D%B4-%E6%99%82%E5%B0%9A%E6%BD%AE%E6%B5%81%E8%A8%AD%E8%A8%88%E6%AC%BE%E7%B6%81%E5%B8%B6%E9%A2%A8%E8%BB%8D%E9%9D%B4-%E6%96%B9%E4%BE%BF%E5%81%B4%E6%8B%89%E9%8D%8A%E4%B8%AD%E7%AD%92%E9%9D%B4-%E7%B6%81%E5%B8%B6%E9%A2%A8%E9%A6%AC%E4%B8%81%E9%9D%B4-%E6%AD%A3%E5%B8%B8-i.179129213.13437612125?extraParams=%7B%22display_model_id%22%3A187331545102%7D",
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
