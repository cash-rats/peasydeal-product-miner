package runner

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveRunnerWorkDir(explicit string) string {
	if clean := strings.TrimSpace(explicit); clean != "" {
		return clean
	}
	if envDir := strings.TrimSpace(os.Getenv("RUNNER_WORKDIR")); envDir != "" {
		return envDir
	}

	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return ""
	}

	if root := findProjectRootFrom(wd); root != "" {
		return root
	}
	return wd
}

func findProjectRootFrom(start string) string {
	current := filepath.Clean(start)
	for {
		if fileExists(filepath.Join(current, "go.mod")) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
