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
	cfg    *config.Config
	runner *runner.Runner
	logger *zap.SugaredLogger
}

type RunResult struct {
	OutPath string        `json:"out_path"`
	Result  runner.Result `json:"result"`
}

type NewCrawlFunctionParams struct {
	fx.In

	Cfg    *config.Config
	Runner *runner.Runner
	Logger *zap.SugaredLogger
}

func NewCrawlFunction(p NewCrawlFunctionParams) *CrawlFunction {
	return &CrawlFunction{
		logger: p.Logger,
		cfg:    p.Cfg,
		runner: p.Runner,
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

		checkURL, effectiveHost := chromedevtools.VersionURLResolved(ctx, f.cfg.Chrome.DebugHost, f.cfg.Chrome.DebugPort)
		if strings.TrimSpace(f.cfg.Chrome.DebugHost) != "" && effectiveHost != strings.TrimSpace(f.cfg.Chrome.DebugHost) {
			f.logger.Infow("chrome_devtools_host_resolved",
				"from", f.cfg.Chrome.DebugHost,
				"to", effectiveHost,
			)
		}
		if _, err := chromedevtools.CheckReachable(ctx, checkURL, 3*time.Second); err != nil {
			f.logger.Errorw(
				"inngest_step_failed",
				"step", "check-devtools",
				"doing", "check Chrome DevTools is reachable",
				"host", effectiveHost,
				"err", err,
			)
			return nil, inngestgo.NoRetryError(err)
		}

		f.logger.Infoln("✅ done check-devtools")
		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	outDir, err := step.Run(ctx, "resolve-out-dir", func(ctx context.Context) (string, error) {
		f.logger.Infow("🏃🏻 inngest_step",
			"step", "resolve-out-dir",
			"doing", "resolve output directory (default out)",
		)
		outDir := input.Event.Data.OutDir
		if outDir == "" {
			outDir = "out"
		}
		f.logger.Infow(
			"✅ done inngest_step_finished",
			"step", "resolve-out-dir",
			"doing", "resolve output directory (default out)",
			"out", outDir,
		)
		return outDir, nil
	})
	if err != nil {
		return nil, err
	}

	r, err := step.Run(ctx, "run-crawler", func(ctx context.Context) (RunResult, error) {
		f.logger.Infow("🏃🏻 inngest_step",
			"step", "run-crawler",
			"doing", "run crawler (runner.RunOnce)",
		)

		outPath, result, err := f.runner.RunOnce(runner.Options{
			URL:    url,
			OutDir: outDir,
			Tool:   strings.TrimSpace(f.cfg.CrawlTool),
		})

		if err != nil {
			f.logger.Errorw("❌ inngest_crawl_failed",
				"url", url,
				"out_path", outPath,
				"err", err,
			)
			return RunResult{}, inngestgo.NoRetryError(err)
		}

		f.logger.Infoln("✅ inngest_step")

		return RunResult{OutPath: outPath, Result: result}, err
	})
	if err != nil {
		return nil, err
	}

	resp := map[string]any{
		"out_path": r.OutPath,
		"result":   r.Result,
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
