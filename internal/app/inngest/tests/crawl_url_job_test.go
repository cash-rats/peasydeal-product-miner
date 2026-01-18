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

		targetURL := "https://shopee.tw/XEXYMIX-%E9%BB%91%E6%A8%99-360N-%E5%85%A8%E6%96%B9%E4%BD%8D%E6%9F%94%E5%BD%88%E7%BE%8E%E6%84%9F-%E7%B7%8A%E8%BA%AB%E8%A4%B2-XP9157T-%E5%85%B124%E8%89%B2-%E8%B7%91%E6%AD%A5-%E5%81%A5%E8%BA%AB%E8%A4%B2-%E7%91%9C%E7%8F%88%E8%A4%B2-%E9%81%8B%E5%8B%95%E8%A4%B2-9157-i.180059250.5465364785"
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

func TestCrawlURLJobTestSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(CrawlURLJobTestSuite))
}
