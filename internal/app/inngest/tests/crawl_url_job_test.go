package tests

import (
	"log"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CrawlURLJobTestSuite struct {
	suite.Suite
}

func (*CrawlURLJobTestSuite) SetupTest() {
	log.Println("setup~~")
}

func (s *CrawlURLJobTestSuite) TestSendCrawlerURLRequested() {
	log.Printf("aaa")
}

func TestCrawlURLJobTestSuite(t *testing.T) {
	suite.Run(t, new(CrawlURLJobTestSuite))
}
