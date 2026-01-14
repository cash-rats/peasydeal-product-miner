package crawl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"peasydeal-product-miner/config"
	"peasydeal-product-miner/internal/pkg/chromedevtools"
	"peasydeal-product-miner/internal/runner"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const CrawlRequestedEventName = "crawler/url.requested"

type CrawlRequestedEventData struct {
	URL    string `json:"url"`
	OutDir string `json:"out_dir,omitempty"`
}

type CrawlFunction struct {
	logger *zap.SugaredLogger
	cfg    *config.Config
}

type RunResult struct {
	OutPath string        `json:"out_path"`
	Result  runner.Result `json:"result"`
}

type NewCrawlFunctionParams struct {
	fx.In

	Logger *zap.SugaredLogger
	Cfg    *config.Config
}

func NewCrawlFunction(p NewCrawlFunctionParams) *CrawlFunction {
	return &CrawlFunction{
		logger: p.Logger,
		cfg:    p.Cfg,
	}
}

func (f *CrawlFunction) Handle(ctx context.Context, input inngestgo.Input[CrawlRequestedEventData]) (any, error) {
	url := strings.TrimSpace(input.Event.Data.URL)
	if url == "" {
		return nil, inngestgo.NoRetryError(fmt.Errorf("missing url"))
	}

	_, err := step.Run(ctx, "check-devtools", func(ctx context.Context) (any, error) {
		f.logger.Infow("🏃🏻 inngest_step",
			"step", "check-devtools",
			"doing", "check Chrome DevTools is reachable",
		)

		checkURL := chromedevtools.VersionURL(chromedevtools.DefaultHost, f.cfg.Chrome.DebugPort)
		if _, err := chromedevtools.CheckReachable(ctx, checkURL, 3*time.Second); err != nil {
			return nil, err
		}

		f.logger.Infoln("✅ done check-devtools")
		return nil, nil
	})
	if err != nil {
		f.logger.Errorw(
			"inngest_step_failed",
			"step", "check-devtools",
			"doing", "check Chrome DevTools is reachable",
			"err", err,
		)
		return nil, inngestgo.NoRetryError(err)
	}

	outDir, err := step.Run(ctx, "resolve-out-dir", func(ctx context.Context) (string, error) {
		f.logger.Infow("🏃🏻 inngest_step",
			"step", "resolve-out-dir",
			"doing", "resolve output directory (default out)",
		)
		outDir := strings.TrimSpace(input.Event.Data.OutDir)
		if outDir == "" {
			outDir = "out"
		}
		f.logger.Infoln("✅ done resolve-out-dir")
		return outDir, nil
	})
	if err != nil {
		f.logger.Errorw(
			"inngest_step_failed",
			"step", "resolve-out-dir",
			"doing", "resolve output directory (default out)",
			"err", err,
		)
		return nil, inngestgo.NoRetryError(err)
	}
	f.logger.Infow(
		"inngest_step_finished",
		"step", "resolve-out-dir",
		"doing", "resolve output directory (default out)",
	)

	r, err := step.Run(ctx, "run-crawler", func(ctx context.Context) (RunResult, error) {
		f.logger.Infow("🏃🏻 inngest_step",
			"step", "run-crawler",
			"doing", "run crawler (runner.RunOnce)",
		)

		outPath, result, err := runner.RunOnce(runner.Options{
			URL:              url,
			OutDir:           outDir,
			CodexModel:       "gpt-5.2",
			SkipGitRepoCheck: true,
		})

		f.logger.Infoln("✅ inngest_step")

		return RunResult{OutPath: outPath, Result: result}, err
	})

	resp := map[string]any{
		"out_path": r.OutPath,
		"result":   r.Result,
	}

	if err != nil {
		f.logger.Errorw("❌ inngest_crawl_failed",
			"url", url,
			"out_path", r.OutPath,
			"err", err,
		)
		return resp, inngestgo.NoRetryError(err)
	}

	f.logger.Infow("inngest_step_finished",
		"step", "run-crawler",
		"doing", "run crawler (runner.RunOnce)",
	)

	f.logger.Infow("inngest_crawl_finished",
		"url", url,
		"out_path", r.OutPath,
	)

	return resp, nil
}
