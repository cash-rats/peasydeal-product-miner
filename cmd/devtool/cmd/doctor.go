package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"peasydeal-product-miner/internal/envutil"
	"peasydeal-product-miner/internal/pkg/chromedevtools"
)

func newDoctorCmd() *cobra.Command {
	var port string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check DevTools is reachable on localhost",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := chromedevtools.VersionURL(chromedevtools.DefaultHost, port)
			fmt.Println("Checking:", url)

			_, err := chromedevtools.CheckReachable(context.Background(), url, 3*time.Second)
			if err != nil {
				return fmt.Errorf("Chrome DevTools not reachable (is Chrome running with --remote-debugging-port=%s?): %w", port, err)
			}
			fmt.Println("✅ OK: Chrome DevTools reachable.")
			return nil
		},
	}

	cmd.Flags().StringVar(&port, "port", envutil.String(os.Getenv, "CHROME_DEBUG_PORT", "9222"), "Chrome DevTools remote debugging port")
	return cmd
}
