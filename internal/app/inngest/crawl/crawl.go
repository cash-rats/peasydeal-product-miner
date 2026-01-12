package crawl

import (
	"context"
	"fmt"
	"strings"

	"peasydeal-product-miner/internal/runner"
	"peasydeal-product-miner/internal/source"

	"github.com/inngest/inngestgo"
	"go.uber.org/zap"
)

const CrawlRequestedEventName = "crawler/url.requested"

type CrawlRequestedEventData struct {
	URL       string `json:"url"`
	OutDir    string `json:"out_dir,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type CrawlFunction struct {
	logger *zap.SugaredLogger
}

func NewCrawlFunction(logger *zap.SugaredLogger) *CrawlFunction {
	return &CrawlFunction{logger: logger}
}

func (f *CrawlFunction) Handle(ctx context.Context, input inngestgo.Input[CrawlRequestedEventData]) (any, error) {
	url := strings.TrimSpace(input.Event.Data.URL)
	if url == "" {
		return nil, fmt.Errorf("missing url")
	}

	src, err := source.Detect(url)
	if err != nil {
		return nil, err
	}

	outDir := strings.TrimSpace(input.Event.Data.OutDir)
	if outDir == "" {
		outDir = "out"
	}

	opts := runner.Options{
		URL:              url,
		OutDir:           outDir,
		CodexModel:       "gpt-5.1-codex-mini",
		SkipGitRepoCheck: true,
	}

	f.logger.Infow("inngest_crawl_start",
		"event", CrawlRequestedEventName,
		"url", url,
		"source", string(src),
		"request_id", strings.TrimSpace(input.Event.Data.RequestID),
		"run_id", input.InputCtx.RunID,
	)

	outPath, result, err := runner.RunOnce(opts)
	if err != nil {
		f.logger.Errorw("inngest_crawl_failed",
			"url", url,
			"source", string(src),
			"out_path", outPath,
			"err", err,
			"run_id", input.InputCtx.RunID,
		)
		return map[string]any{
			"out_path": outPath,
			"result":   result,
			"source":   string(src),
		}, err
	}

	f.logger.Infow("inngest_crawl_finished",
		"url", url,
		"source", string(src),
		"out_path", outPath,
		"run_id", input.InputCtx.RunID,
	)

	return map[string]any{
		"out_path": outPath,
		"result":   result,
		"source":   string(src),
	}, nil
}
