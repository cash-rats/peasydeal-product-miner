package crawler

import (
	"fmt"

	"peasydeal-product-miner/internal/source"
)

type Crawler interface {
	Source() source.Source
	DefaultPromptFile() string
}

func ForSource(s source.Source) (Crawler, error) {
	switch s {
	case source.Shopee:
		return ShopeeCrawler{}, nil
	case source.Taobao:
		return TaobaoCrawler{}, nil
	default:
		return nil, fmt.Errorf("unsupported source %q", s)
	}
}
