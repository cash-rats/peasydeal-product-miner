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

		targetURL := "https://shopee.tw/SP-A66-%E5%A4%96%E6%8E%A5%E7%A1%AC%E7%A2%9F-1TB-2TB-4TB-5TB-2.5%E5%90%8B-%E8%BB%8D%E8%A6%8F%E9%98%B2%E9%9C%87-%E8%A1%8C%E5%8B%95%E7%A1%AC%E7%A2%9F-%E7%A7%BB%E5%8B%95%E5%BC%8F%E7%A1%AC%E7%A2%9F-HDD-%E9%98%B2%E6%B0%B4-%E5%BB%A3%E7%A9%8E-i.77690258.15602180381"
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
