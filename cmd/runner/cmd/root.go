package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
	"peasydeal-product-miner/internal/runner"
)

func newRootCmd() *cobra.Command {
	var (
		url        string
		promptFile string
		outDir     string
		codexCmd   string
	)

	rootCmd := &cobra.Command{
		Use:           "runner",
		Short:         "Run one crawl and write a JSON result file",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(url) == "" {
				_ = cmd.Help()
				return errUsage
			}

			outPath, _, err := runner.RunOnce(runner.Options{
				URL:              url,
				PromptFile:       promptFile,
				OutDir:           outDir,
				CodexCmd:         codexCmd,
				SkipGitRepoCheck: envutil.Bool(os.Getenv, "CODEX_SKIP_GIT_REPO_CHECK", false),
			})

			if outPath != "" {
				fmt.Fprintln(cmd.OutOrStdout(), outPath)
			}
			if err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "ERROR:", err)
			}
			// Preserve prior behavior: if an output file was produced, treat crawl errors as non-fatal.
			if err != nil && outPath == "" {
				return err
			}
			return nil
		},
	}

	rootCmd.Flags().StringVar(&url, "url", "", "Shopee product URL")
	rootCmd.Flags().StringVar(&promptFile, "prompt-file", "", "Prompt template file path (optional; auto-selected by URL when empty)")
	rootCmd.Flags().StringVar(&outDir, "out-dir", "out", "Output directory for result JSON")
	rootCmd.Flags().StringVar(&codexCmd, "codex-cmd", envutil.String(os.Getenv, "CODEX_CMD", "codex"), "Codex CLI command name/path")

	if err := rootCmd.MarkFlagRequired("url"); err != nil {
		// Cobra's required-flag implementation is pflag-based; failure here indicates programmer error.
		panic(errors.New("failed to mark required flag: url"))
	}

	return rootCmd
}
