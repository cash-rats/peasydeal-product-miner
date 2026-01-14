package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func userHomeDir() string {
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}
	return "."
}

func findFirstInPath(candidates []string) (string, error) {
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("could not find Chrome/Chromium in PATH (tried: %s)", strings.Join(candidates, ", "))
}
