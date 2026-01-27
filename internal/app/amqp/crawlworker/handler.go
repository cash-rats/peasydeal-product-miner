package crawlworker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"peasydeal-product-miner/config"
	productdrafts "peasydeal-product-miner/internal/app/inngest/dao"
	"peasydeal-product-miner/internal/pkg/chromedevtools"
	"peasydeal-product-miner/internal/runner"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

type CrawlHandler struct {
	cfg    *config.Config
	runner *runner.Runner
	store  *productdrafts.ProductDraftStore
	logger *zap.SugaredLogger
}

type NewCrawlHandlerParams struct {
	fx.In

	Cfg    *config.Config
	Runner *runner.Runner
	Store  *productdrafts.ProductDraftStore
	Logger *zap.SugaredLogger
}

func NewCrawlHandler(p NewCrawlHandlerParams) *CrawlHandler {
	return &CrawlHandler{
		cfg:    p.Cfg,
		runner: p.Runner,
		store:  p.Store,
		logger: p.Logger,
	}
}

func (h *CrawlHandler) Handle(ctx context.Context, msg CrawlRequestedEnvelope) error {
	url := strings.TrimSpace(msg.Data.URL)
	if url == "" {
		return fmt.Errorf("missing url")
	}
	if strings.TrimSpace(msg.EventID) == "" {
		return fmt.Errorf("missing event_id")
	}
	if strings.TrimSpace(msg.EventName) != "" && msg.EventName != "crawler/url.requested" {
		return fmt.Errorf("unexpected event_name: %s", msg.EventName)
	}

	checkURL, effectiveHost := chromedevtools.VersionURLResolved(ctx, h.cfg.Chrome.DebugHost, h.cfg.Chrome.DebugPort)
	if strings.TrimSpace(h.cfg.Chrome.DebugHost) != "" && effectiveHost != strings.TrimSpace(h.cfg.Chrome.DebugHost) {
		h.logger.Infow("chrome_devtools_host_resolved",
			"from", h.cfg.Chrome.DebugHost,
			"to", effectiveHost,
		)
	}
	if _, err := chromedevtools.CheckReachable(ctx, checkURL, 3*time.Second); err != nil {
		h.logger.Errorw(
			"crawlworker_check_devtools_failed",
			"event_id", msg.EventID,
			"host", effectiveHost,
			"err", err,
		)
		return err
	}

	outDir := strings.TrimSpace(msg.Data.OutDir)
	if outDir == "" {
		outDir = "out"
	}

	outPath, result, err := h.runner.RunOnce(runner.Options{
		URL:    url,
		OutDir: outDir,
		Tool:   h.cfg.CrawlTool,
	})
	if err != nil {
		h.logger.Errorw("crawlworker_run_crawler_failed",
			"event_id", msg.EventID,
			"url", url,
			"out_path", outPath,
			"err", err,
		)
		// Intentionally swallow crawler failures so we can persist the failure result (same as Inngest).
	} else {
		h.logger.Infow("crawlworker_run_crawler_ok",
			"event_id", msg.EventID,
			"url", url,
			"out_path", outPath,
		)
	}

	draftID, err := h.store.UpsertFromCrawlResult(ctx, productdrafts.UpsertFromCrawlResultInput{
		EventID:   msg.EventID,
		CreatedBy: "rabbitmq",
		URL:       url,
		Result:    result,
	})
	if err != nil {
		h.logger.Errorw("crawlworker_persist_product_draft_failed",
			"event_id", msg.EventID,
			"url", url,
			"err", err,
		)
		return err
	}

	h.logger.Infow("crawlworker_finished",
		"event_id", msg.EventID,
		"url", url,
		"draft_id", draftID,
		"out_path", outPath,
	)

	return nil
}
