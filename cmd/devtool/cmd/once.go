package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
	runnerPkg "peasydeal-product-miner/internal/runner"
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

			outPath, _, err := runnerPkg.RunOnce(runnerPkg.Options{
				URL:              url,
				PromptFile:       promptFile,
				OutDir:           outDir,
				Tool:             tool,
				Cmd:              envutil.String(os.Getenv, "CODEX_CMD", ""),
				Model:            model,
				SkipGitRepoCheck: envutil.Bool(os.Getenv, "CODEX_SKIP_GIT_REPO_CHECK", false),
			})
			if outPath != "" {
				fmt.Println(outPath)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "ERROR:", err)
			}
			// If we managed to write an output file, treat crawl errors as non-fatal.
			if err != nil && outPath == "" {
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
