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
			vp.Set("inngest.dev", "1")
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

		evtID, err := s.client.Send(ctx, inngestgo.Event{
			Name: crawl.CrawlRequestedEventName,
			Data: map[string]any{
				"url":        "https://shopee.tw/%E3%80%90%E4%B8%8B%E6%AE%BA5%E5%85%83%E2%9C%A8%E5%85%8D%E9%81%8B%E3%80%91Kitty-Licks-%E7%94%9C%E7%94%9C%E8%B2%93%E8%82%89%E6%B3%A5-%E8%B2%93%E8%82%89%E6%B3%A5-%E8%B2%93%E6%A2%9D-11%E7%A8%AE%E5%8F%A3%E5%91%B3-%E8%82%89%E6%B3%A5%E6%A2%9D-%E9%9B%B6%E9%A3%9F-%E8%B2%93%E6%A2%9D-%E5%AF%B5%E7%89%A9%E9%9B%B6%E9%A3%9F-%E8%B2%93%E6%B3%A5-i.9187906.18379845026?extraParams=%7B%22display_model_id%22%3A138621311710%2C%22model_selection_logic%22%3A3%7D",
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
