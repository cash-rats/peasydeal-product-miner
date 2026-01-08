package crawler

import "peasydeal-product-miner/internal/source"

type ShopeeCrawler struct{}

func (ShopeeCrawler) Source() source.Source { return source.Shopee }

func (ShopeeCrawler) DefaultPromptFile() string { return "config/prompt.shopee.product.txt" }

