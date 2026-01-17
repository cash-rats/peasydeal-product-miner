package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	appfx "peasydeal-product-miner/internal/app/fx"
	"peasydeal-product-miner/internal/envutil"
	"peasydeal-product-miner/internal/runner"
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
			if strings.TrimSpace(url) == "" {
				return errors.New("missing required flag: --url")
			}

			app := fx.New(
				fx.Supply(
					runnerPkg.CodexRunnerConfig{
						Cmd:              "codex",
						Model:            model,
						SkipGitRepoCheck: true,
					},
					runnerPkg.GeminiRunnerConfig{
						Cmd:   "gemini",
						Model: model,
					},
				),

				appfx.CoreAppOptions,
				runnerFx.AsRunner(runnerPkg.NewCodexRunner),
				runnerFx.AsRunner(runnerPkg.NewGeminiRunner),

				fx.Provide(
					runner.NewRunners,
					runnerPkg.NewService,
				),

				fx.Invoke(func(svc *runner.Service) {
					outPath, _, err := svc.RunOnce(runnerPkg.Options{
						URL:        url,
						PromptFile: promptFile,
						OutDir:     outDir,
						Tool:       tool,
					})

					if outPath != "" {
						fmt.Println(outPath)
					}

					if err != nil {
						fmt.Fprintln(os.Stderr, "ERROR:", err)
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
	cmd.Flags().StringVar(&model, "model", envutil.String(os.Getenv, "CODEX_MODEL", ""), "Model override (optional; also via CODEX_MODEL)")
	cmd.Flags().StringVar(&tool, "tool", "codex", "Tool to use (codex or gemini)")
	return cmd
}
