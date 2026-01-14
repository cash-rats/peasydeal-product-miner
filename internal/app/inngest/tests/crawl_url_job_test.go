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

		targetURL := "https://shopee.tw/%E3%80%90%E9%99%84%E7%99%BC%E7%A5%A8%E3%80%9124H%E6%A1%8C%E5%A2%8A-%E9%80%8F%E6%98%8E%E7%A3%A8%E9%82%8A-%E9%98%B2%E6%B0%B4%E9%98%B2%E6%B2%B9-%E5%AE%A2%E8%A3%BD%E8%A3%81%E5%89%AA-%E5%8E%9A%E5%BA%A65mm-%E5%A4%9A%E9%81%B8%E9%A4%90%E6%A1%8C%E5%A2%8A-%E5%9C%93%E6%A1%8C%E5%A2%8A%E8%8C%B6%E5%87%A0%E5%A2%8A-%E9%AB%98%E7%B4%9A%E6%84%9F%E5%8F%AF%E8%A3%81%E5%89%AA%E7%84%A1%E5%91%B3%E6%A1%8C%E5%A2%8A-%E9%98%B2%E7%87%99%E9%98%B2%E6%B2%B9-i.279216161.21065663294?extraParams=%7B%22display_model_id%22%3A158245746215%2C%22model_selection_logic%22%3A3%7D"
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
