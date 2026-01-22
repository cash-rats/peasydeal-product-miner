package cmd

import (
	"errors"

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
	)

	cmd := &cobra.Command{
		Use:   "once",
		Short: "Crawl one URL on the host (fast loop)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				return errors.New("missing required flag: --url")
			}

			app := fx.New(
				fx.Supply(
					runnerPkg.CodexRunnerConfig{
						Cmd:              "codex",
						Model:            "gpt-5.2",
						SkipGitRepoCheck: true,
					},
					runnerPkg.GeminiRunnerConfig{
						Cmd:   "gemini",
						Model: "",
					},
				),

				appfx.CoreAppOptions,
				runnerFx.AsRunner(runnerPkg.NewCodexRunner),
				runnerFx.AsRunner(runnerPkg.NewGeminiRunner),

				fx.Provide(
					runnerPkg.NewRunners,
					runnerPkg.NewRunner,
				),

				fx.Invoke(func(
					r *runnerPkg.Runner,
					logger *zap.SugaredLogger,
					cfg *config.Config,
				) {
					outPath, _, err := r.RunOnce(
						runnerPkg.Options{
							URL:        url,
							PromptFile: promptFile,
							OutDir:     outDir,
							Tool:       tool,
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

	cmd.Flags().StringVar(&url, "url", "", "Shopee product URL")
	cmd.Flags().StringVar(&promptFile, "prompt-file", "", "Prompt template file path (optional; auto-selected by URL when empty)")
	cmd.Flags().StringVar(&outDir, "out-dir", "out", "Output directory for result JSON")
	cmd.Flags().StringVar(&model, "model", "", "Model override for the selected tool (optional; defaults to CODEX_MODEL or GEMINI_MODEL)")
	cmd.Flags().StringVar(&tool, "tool", "codex", "Tool to use (codex or gemini)")
	return cmd
}
