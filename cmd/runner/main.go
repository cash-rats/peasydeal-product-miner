package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"peasydeal-product-miner/internal/runner"
)

func main() {
	var (
		url        = flag.String("url", "", "Shopee product URL")
		promptFile = flag.String("prompt-file", filepath.Join("config", "prompt.product.txt"), "Prompt template file path")
		outDir     = flag.String("out-dir", "out", "Output directory for result JSON")
		codexCmd   = flag.String("codex-cmd", getenvDefault("CODEX_CMD", "codex"), "Codex CLI command name/path")
	)
	flag.Parse()

	outPath, _, err := runner.RunOnce(runner.Options{
		URL:        *url,
		PromptFile: *promptFile,
		OutDir:     *outDir,
		CodexCmd:   *codexCmd,
	})

	// Preserve prior behavior: always write a result JSON file if possible and print its path.
	// Non-fatal crawl errors are represented in the JSON as status="error".
	if outPath != "" {
		fmt.Println(outPath)
	}
	if err != nil && outPath == "" {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getenvDefault(key string, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

