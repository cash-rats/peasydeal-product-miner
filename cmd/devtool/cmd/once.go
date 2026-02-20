package cmd

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"peasydeal-product-miner/config"
	appfx "peasydeal-product-miner/internal/app/fx"
	runnerPkg "peasydeal-product-miner/internal/runner"
	runnerFx "peasydeal-product-miner/internal/runner/fx"
)

func newOnceCmd() *cobra.Command {
	var (
		url        string
		promptFile string
		outDir     string
		model      string
		tool       string
		promptMode string
		skillName  string
		runID      string
	)

	cmd := &cobra.Command{
		Use:   "once",
		Short: "Crawl one URL on the host (fast loop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(url) == "" {
				return errors.New("missing required flag: --url")
			}

			app := fx.New(
				appfx.CoreAppOptions,
				runnerFx.AsRunner(runnerPkg.NewCodexRunner),
				runnerFx.AsRunner(runnerPkg.NewGeminiRunner),

				fx.Provide(
					func(cfg *config.Config, logger *zap.SugaredLogger) runnerPkg.CodexRunnerConfig {
						return runnerPkg.CodexRunnerConfig{
							Cmd:              "codex",
							Model:            cfg.CodexModel,
							SkipGitRepoCheck: true,
							Logger:           logger,
						}
					},
					func(cfg *config.Config, logger *zap.SugaredLogger) runnerPkg.GeminiRunnerConfig {
						return runnerPkg.GeminiRunnerConfig{
							Cmd:    "gemini",
							Model:  cfg.GeminiModel,
							Logger: logger,
						}
					},
					runnerPkg.NewRunners,
					runnerPkg.NewRunner,
				),

				fx.Invoke(func(
					r *runnerPkg.Runner,
					logger *zap.SugaredLogger,
				) {
					outPath, _, err := r.RunOnce(
						runnerPkg.Options{
							URL:        url,
							PromptFile: promptFile,
							OutDir:     outDir,
							Tool:       tool,
							PromptMode: promptMode,
							SkillName:  skillName,
							RunID:      resolveRunID(runID),
						},
					)

					if err != nil {
						logger.Errorw("❌ Woops, failed to crawl",
							"output_path", outPath,
							"err", err,
						)
						return
					}

					if outPath != "" {
						logger.Infow(
							"✅ crawl success",
							"output_path", outPath,
						)
					}
				}),
			)

			if err := app.Start(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "Product URL (Shopee/Taobao/Tmall)")
	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "Prompt template file path (optional; auto-selected by URL when empty)")
	cmd.Flags().StringVar(&outDir, "out-dir", "out", "Output directory for result JSON")
	cmd.Flags().StringVar(&model, "model", "", "Model override for the selected tool (optional; defaults to CODEX_MODEL/GEMINI_MODEL config)")
	cmd.Flags().StringVar(&tool, "tool", "codex", "Tool to use (codex or gemini)")
	cmd.Flags().StringVar(&promptMode, "prompt-mode", "", "Prompt mode: legacy or skill (optional; defaults to CRAWL_PROMPT_MODE or legacy)")
	cmd.Flags().StringVar(&skillName, "skill-name", "", "Skill name override when --prompt-mode=skill (optional)")
	cmd.Flags().StringVar(&runID, "run-id", "", "Run ID for artifact correlation (optional; auto-generated when empty)")
	return cmd
}

func resolveRunID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return raw
	}
	return uuid.NewString()
}
