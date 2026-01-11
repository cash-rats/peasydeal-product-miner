package crawler

import "peasydeal-product-miner/internal/source"

type TaobaoCrawler struct{}

func (TaobaoCrawler) Source() source.Source { return source.Taobao }

func (TaobaoCrawler) DefaultPromptFile() string { return "config/prompt.taobao.product.txt" }
