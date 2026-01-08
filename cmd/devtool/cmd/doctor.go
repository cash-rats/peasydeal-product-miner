package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
)

func newDoctorCmd() *cobra.Command {
	var port string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check DevTools is reachable on localhost",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("http://127.0.0.1:%s/json/version", port)
			fmt.Println("Checking:", url)

			client := &http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(url)
			if err != nil {
				return fmt.Errorf("Chrome DevTools not reachable (is Chrome running with --remote-debugging-port=%s?): %w", port, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return fmt.Errorf("unexpected status %s from %s", resp.Status, url)
			}
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*32))
			if len(bytesTrimSpace(b)) == 0 {
				return fmt.Errorf("empty response from %s", url)
			}
			fmt.Println("✅ OK: Chrome DevTools reachable.")
			return nil
		},
	}

	cmd.Flags().StringVar(&port, "port", envutil.String(os.Getenv, "CHROME_DEBUG_PORT", "9222"), "Chrome DevTools remote debugging port")
	return cmd
}

