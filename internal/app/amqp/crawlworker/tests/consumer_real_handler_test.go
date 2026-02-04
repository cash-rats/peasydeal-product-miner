package tests

import (
	"context"
	"encoding/json"
	"log"
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

	log.Printf("~~ rabbitURL %v", rabbitURL)

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
			URL:    "https://shopee.tw/-%E6%82%A0%E8%A5%BF%E5%B0%87-%E6%97%A5%E6%9C%AC-NISSIN-%E6%97%A5%E6%B8%85-%E6%9D%AF%E9%BA%B5-%E6%B5%B7%E9%AE%AE%E6%9D%AF%E9%BA%B5-cup-noodle-%E6%B3%A1%E9%BA%B5-%E8%BE%A3%E9%86%AC%E6%B2%B9%E5%91%B3-%E8%BE%A3%E7%95%AA%E8%8C%84%E5%91%B3-3%E7%A8%AE%E5%91%B3%E5%99%8C-%E6%93%94%E6%93%94%E9%BA%B5-i.2293056.45953187011?extraParams=%7B%22display_model_id%22%3A435405193306%2C%22model_selection_logic%22%3A3%7D&sp_atk=d56bf2c0-39ab-41aa-b422-95ff9a766d22&xptdk=d56bf2c0-39ab-41aa-b422-95ff9a766d22",
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
