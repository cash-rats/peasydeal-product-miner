package crawl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"peasydeal-product-miner/config"
	productdrafts "peasydeal-product-miner/internal/app/amqp/productdrafts"
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
	store  *productdrafts.ProductDraftStore
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
	Store  *productdrafts.ProductDraftStore
	Logger *zap.SugaredLogger
}

func NewCrawlFunction(p NewCrawlFunctionParams) *CrawlFunction {
	return &CrawlFunction{
		logger: p.Logger,
		cfg:    p.Cfg,
		runner: p.Runner,
		store:  p.Store,
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
			"doing", "check Chrome DevTools is reachable (mocked)",
		)

		// DEBUG MODE: Skip real Chrome DevTools checks to isolate Inngest execution & logging.
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
		f.logger.Infow("🏃🏻 resolve-out-dir",
			"step", "resolve-out-dir",
			"doing", "resolve output directory (default out)",
		)

		outDir := input.Event.Data.OutDir
		if outDir == "" {
			outDir = "out"
		}

		f.logger.Infow(
			"✅ done resolve-out-dir",
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
		f.logger.Infow("🏃🏻 run-crawler",
			"step", "run-crawler",
			"doing", "run crawler (runner.RunOnce) (mocked)",
		)

		outPath, result, err := f.runner.RunOnce(runner.Options{
			URL:    url,
			OutDir: outDir,
			Tool:   f.cfg.CrawlTool,
		})

		if err != nil {
			f.logger.Errorw("❌ inngest_crawl_failed",
				"url", url,
				"out_path", outPath,
				"err", err,
			)
			// Do not fail the step; we still want to persist the failure result.
			return RunResult{OutPath: outPath, Result: result}, nil
		}

		f.logger.Infoln("✅ run-crawler")

		return RunResult{OutPath: outPath, Result: result}, nil
	})
	if err != nil {
		return nil, inngestgo.NoRetryError(err)
	}

	draftID, err := step.Run(ctx, "persist-product-draft", func(ctx context.Context) (string, error) {
		status, _ := r.Result["status"].(string)
		resultErr, _ := r.Result["error"].(string)

		f.logger.Infow("persist-product-draft",
			"step", "persist-product-draft",
			"doing", "upsert crawl result into turso sqlite product_drafts (mocked)",
			"result_status", status,
			"result_keys", len(r.Result),
			"result_error", resultErr,
			"out_path", r.OutPath,
		)

		eventID := ""
		if input.Event.ID != nil {
			eventID = strings.TrimSpace(*input.Event.ID)
		}

		id, err := f.store.UpsertFromCrawlResult(ctx, productdrafts.UpsertFromCrawlResultInput{
			EventID:   eventID,
			CreatedBy: "inngest",
			URL:       url,
			Result:    r.Result,
		})
		if err != nil {
			f.logger.Errorw(
				"❌ inngest_step_failed",
				"step", "persist-product-draft",
				"doing", "upsert crawl result into turso sqlite product_drafts",
				"err", err,
			)
			return "", inngestgo.NoRetryError(err)
		}

		f.logger.Infow(
			"✅ done persist-product-draft",
			"step", "persist-product-draft",
			"doing", "upsert crawl result into turso sqlite product_drafts (mocked)",
			"draft_id", id,
		)

		return id, nil
	})
	if err != nil {
		return nil, inngestgo.NoRetryError(err)
	}

	resp := map[string]any{
		"draft_id": draftID,
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
