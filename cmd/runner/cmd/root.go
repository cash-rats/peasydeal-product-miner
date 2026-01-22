package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
)

func newRootCmd() *cobra.Command {
	var (
		url        string
		promptFile string
		outDir     string
		tool       string
		cmdName    string
		model      string
		codexCmd   string
	)

	rootCmd := &cobra.Command{
		Use:           "runner",
		Short:         "Run one crawl and write a JSON result file",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				_ = cmd.Help()
				return errUsage
			}
			// if strings.TrimSpace(url) == "" {
			// 	_ = cmd.Help()
			// }
			//
			// toolName := strings.TrimSpace(tool)
			// if toolName == "" {
			// 	toolName = envutil.String(os.Getenv, "CRAWL_TOOL", "codex")
			// }
			//
			// resolvedCmd := strings.TrimSpace(cmdName)
			// if resolvedCmd == "" {
			// 	switch toolName {
			// 	case "codex":
			// 		resolvedCmd = envutil.String(os.Getenv, "CODEX_CMD", "codex")
			// 	case "gemini":
			// 		resolvedCmd = envutil.String(os.Getenv, "GEMINI_CMD", "gemini")
			// 	}
			// }
			// if resolvedCmd == "" {
			// 	resolvedCmd = strings.TrimSpace(codexCmd)
			// }
			//
			// resolvedModel := strings.TrimSpace(model)
			// if resolvedModel == "" {
			// 	switch toolName {
			// 	case "codex":
			// 		resolvedModel = strings.TrimSpace(os.Getenv("CODEX_MODEL"))
			// 	case "gemini":
			// 		resolvedModel = strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
			// 	}
			// }
			//
			// outPath, _, err := runner.RunOnce(runner.Options{
			// 	URL:              url,
			// 	PromptFile:       promptFile,
			// 	OutDir:           outDir,
			// 	Tool:             toolName,
			// 	Cmd:              resolvedCmd,
			// 	Model:            resolvedModel,
			// 	SkipGitRepoCheck: envutil.Bool(os.Getenv, "CODEX_SKIP_GIT_REPO_CHECK", false),
			// })
			//
			// if outPath != "" {
			// 	fmt.Fprintln(cmd.OutOrStdout(), outPath)
			// }
			// if err != nil {
			// 	fmt.Fprintln(cmd.ErrOrStderr(), "ERROR:", err)
			// }
			// // Preserve prior behavior: if an output file was produced, treat crawl errors as non-fatal.
			// if err != nil && outPath == "" {
			// 	return err
			// }
			// return nil
			return nil
		},
	}

	rootCmd.Flags().StringVar(&url, "url", "", "Shopee product URL")
	rootCmd.Flags().StringVar(&promptFile, "prompt-file", "", "Prompt template file path (optional; auto-selected by URL when empty)")
	rootCmd.Flags().StringVar(&outDir, "out-dir", "out", "Output directory for result JSON")
	rootCmd.Flags().StringVar(&tool, "tool", envutil.String(os.Getenv, "CRAWL_TOOL", "codex"), "Tool to use (codex or gemini)")
	rootCmd.Flags().StringVar(&cmdName, "cmd", "", "Tool binary name/path (defaults to CODEX_CMD/GEMINI_CMD based on --tool)")
	rootCmd.Flags().StringVar(&model, "model", "", "Model override for the selected tool (defaults to CODEX_MODEL/GEMINI_MODEL based on --tool)")
	rootCmd.Flags().StringVar(&codexCmd, "codex-cmd", envutil.String(os.Getenv, "CODEX_CMD", "codex"), "Deprecated alias for --cmd (Codex CLI command name/path)")

	if err := rootCmd.MarkFlagRequired("url"); err != nil {
		// Cobra's required-flag implementation is pflag-based; failure here indicates programmer error.
		panic(errors.New("failed to mark required flag: url"))
	}

	return rootCmd
}
