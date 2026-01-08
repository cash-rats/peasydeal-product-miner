package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
)

func newChromeCmd() *cobra.Command {
	var (
		port       string
		profileDir string
	)

	cmd := &cobra.Command{
		Use:   "chrome",
		Short: "Start Chrome with DevTools enabled (dedicated profile)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(port) == "" {
				return errors.New("missing --port")
			}
			if strings.TrimSpace(profileDir) == "" {
				return errors.New("missing --profile-dir")
			}
			if err := os.MkdirAll(profileDir, 0o755); err != nil {
				return err
			}

			switch runtime.GOOS {
			case "darwin":
				// macOS: use `open` so it starts as a normal app instance.
				c := exec.Command("open", "-na", "Google Chrome", "--args",
					"--remote-debugging-port="+port,
					"--user-data-dir="+profileDir,
				)
				if err := c.Start(); err != nil {
					return err
				}
			case "linux":
				bin, err := findFirstInPath([]string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"})
				if err != nil {
					return err
				}
				c := exec.Command(bin,
					"--remote-debugging-port="+port,
					"--user-data-dir="+profileDir,
				)
				c.Stdout = io.Discard
				c.Stderr = io.Discard
				if err := c.Start(); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported OS for auto-launch: %s (start Chrome manually with --remote-debugging-port and --user-data-dir)", runtime.GOOS)
			}

			fmt.Printf("Chrome launch requested (port=%s, profile=%s)\n", port, profileDir)
			fmt.Printf("DevTools check: http://127.0.0.1:%s/json/version\n", port)
			return nil
		},
	}

	defPort := envutil.String(os.Getenv, "CHROME_DEBUG_PORT", "9222")
	defProfile := envutil.String(os.Getenv, "CHROME_PROFILE_DIR", filepath.Join(userHomeDir(), "chrome-mcp-profiles", "shopee"))

	cmd.Flags().StringVar(&port, "port", defPort, "Chrome DevTools remote debugging port")
	cmd.Flags().StringVar(&profileDir, "profile-dir", defProfile, "Dedicated Chrome profile directory (non-default)")
	return cmd
}
