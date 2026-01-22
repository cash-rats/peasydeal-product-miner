package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
	"peasydeal-product-miner/internal/pkg/chromedevtools"
)

func newDockerDoctorCmd() *cobra.Command {
	var (
		port     string
		tool     string
		authFile string
	)

	cmd := &cobra.Command{
		Use:   "docker-doctor",
		Short: "Check Chrome + tool auth for Docker runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := chromedevtools.VersionURL(chromedevtools.DefaultHost, port)
			_, err := chromedevtools.CheckReachable(context.Background(), url, 3*time.Second)
			if err != nil {
				return fmt.Errorf("host Chrome DevTools not reachable at %s (start it via `make dev-chrome` and login/solve CAPTCHA if needed): %w", url, err)
			}
			fmt.Printf("✅ Chrome ready: DevTools reachable at %s\n", url)

			toolName := strings.TrimSpace(tool)
			if toolName == "" {
				toolName = "codex"
			}

			switch toolName {
			case "gemini":
				fmt.Println("ℹ️  Gemini selected: skipping Codex auth check.")
				fmt.Println("OK: Host Chrome DevTools reachable.")
				return nil
			case "codex":
				// Continue below.
			default:
				return fmt.Errorf("unknown tool: %s (expected codex or gemini)", toolName)
			}

			info, err := os.Stat(authFile)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("Codex auth missing at %s (run `make docker-login` once)", authFile)
				}
				return err
			}
			if info.Size() == 0 {
				return fmt.Errorf("Codex auth file is empty at %s (run `make docker-login` again)", authFile)
			}
			b, err := os.ReadFile(authFile)
			if err != nil {
				return err
			}
			var tmp any
			if err := json.Unmarshal(b, &tmp); err != nil {
				return fmt.Errorf("Codex auth file is not valid JSON at %s (run `make docker-login` again): %w", authFile, err)
			}

			fmt.Printf("✅ Codex ready: auth present at %s\n", authFile)
			fmt.Println("OK: Host Chrome DevTools reachable and Codex auth present for Docker.")
			return nil
		},
	}

	cmd.Flags().StringVar(&port, "port", envutil.String(os.Getenv, "CHROME_DEBUG_PORT", "9222"), "Chrome DevTools remote debugging port on the host")
	cmd.Flags().StringVar(&tool, "tool", envutil.String(os.Getenv, "CRAWL_TOOL", "codex"), "Tool to use (codex or gemini)")
	cmd.Flags().StringVar(&authFile, "auth-file", filepath.Join("codex", ".codex", "auth.json"), "Path to Codex auth.json persisted for Docker runs")
	return cmd
}
