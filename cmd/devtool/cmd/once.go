package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	runnerPkg "peasydeal-product-miner/internal/runner"
	"peasydeal-product-miner/internal/envutil"
)

func newOnceCmd() *cobra.Command {
	var (
		url        string
		promptFile string
		outDir     string
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
				CodexCmd:         envutil.String(os.Getenv, "CODEX_CMD", "codex"),
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
	return cmd
}
