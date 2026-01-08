package cmd

import (
	"errors"
	"fmt"
	"os"
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
