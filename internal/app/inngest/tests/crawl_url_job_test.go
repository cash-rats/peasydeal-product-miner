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

		targetURL := "https://shopee.tw/%E3%80%90BRITA%E5%AE%98%E6%96%B9%E3%80%91%E5%8E%BB%E6%B0%B4%E5%9E%A2%E5%B0%88%E5%AE%B612%E5%85%A5%E7%89%B9%E6%83%A0%E7%B5%84%E3%80%8A%E8%B4%88%E6%B0%B4%E5%A3%BA%E3%80%8B-i.270022711.11755660099"
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
