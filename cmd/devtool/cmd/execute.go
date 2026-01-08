package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var errUsage = errors.New("usage")

func Execute() int {
	root := newRootCmd()
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)

	if err := root.Execute(); err != nil {
		if errors.Is(err, errUsage) {
			return 2
		}
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		// Preserve prior behavior: show usage for unknown commands too.
		if strings.HasPrefix(err.Error(), "unknown command") {
			_ = root.Help()
			return 2
		}
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "devtool",
		Short:         "Developer helper commands",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return errUsage
		},
	}

	rootCmd.AddCommand(
		newChromeCmd(),
		newDoctorCmd(),
		newDockerDoctorCmd(),
		newOnceCmd(),
	)
	return rootCmd
}

