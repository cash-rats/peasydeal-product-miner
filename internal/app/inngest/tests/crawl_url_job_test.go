package tests

import (
	"context"
	"testing"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/app/inngest/crawl"
	pkginngest "peasydeal-product-miner/internal/pkg/inngest"

	"github.com/inngest/inngestgo"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
	"go.uber.org/fx"
)

type CrawlURLJobTestSuite struct {
	suite.Suite

	app    *fx.App
	client inngestgo.Client
}

func (s *CrawlURLJobTestSuite) SetupTest() {
	var client inngestgo.Client

	s.app = fx.New(
		fx.Provide(func() *viper.Viper {
			vp := config.NewViper()
			vp.Set("inngest.dev", "http://localhost:8288")
			vp.Set("inngest.app_id", "test-app")
			return vp
		}),
		fx.Provide(config.NewConfig),
		fx.Provide(pkginngest.NewInngestClient),
		fx.Populate(&client),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.Require().NoError(s.app.Start(ctx))
	s.client = client
}

func (s *CrawlURLJobTestSuite) TearDownTest() {
	if s.app == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	s.Require().NoError(s.app.Stop(ctx))
}

func (s *CrawlURLJobTestSuite) TestSendCrawlerURLRequested() {
	s.T().Parallel()

	s.Run("e2e_send", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		targetURL := "https://shopee.tw/VOLA%E7%B6%AD%E8%8F%88-PH5.5%E6%8A%97%E8%8F%8C%E8%A6%AA%E8%A6%AA%E8%A4%B2-%E8%8E%AB%E4%BB%A3%E7%88%BE%E4%B8%AD%E8%85%B0-%E5%85%A7%E8%A4%B2-%E5%A5%B3%E6%80%A7%E5%85%A7%E8%A4%B2-%E6%8A%97%E8%8F%8C%E5%85%A7%E8%A4%B2-%E6%8A%91%E8%8F%8C%E5%85%A7%E8%A4%B2-%E5%A5%B3%E7%94%9F%E5%85%A7%E8%A4%B2-%E4%B8%AD%E8%85%B0%E5%85%A7%E8%A4%B2-%E9%80%8F%E6%B0%A3%E5%85%A7%E8%A4%B2-%E5%A4%A7%E5%B0%BA%E7%A2%BC-i.129648146.21676952318"
		eventID, err := crawl.EventIDForURL(targetURL)
		s.Require().NoError(err)

		evtID, err := s.client.Send(ctx, inngestgo.Event{
			ID:   inngestgo.StrPtr(eventID),
			Name: crawl.CrawlRequestedEventName,
			Data: map[string]any{
				"url":        targetURL,
				"out_dir":    "out",
				"request_id": "e2e-test",
			},
			Timestamp: inngestgo.Timestamp(time.Now()),
		})
		s.Require().NoError(err)
		s.NotEmpty(evtID)
	})
}

/*
cmd:
dotenvx run -f .env -- go test -run TestCrawlURLJobTestSuite ./internal/app/inngest/tests/crawl_url_job_test.go -testify.m TestSendCrawlerURLRequested -v
*/
func TestCrawlURLJobTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CrawlURLJobTestSuite))
}
