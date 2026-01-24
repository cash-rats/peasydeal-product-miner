package runner

import (
	"os"
	"path/filepath"
)

func resolveHomePath(rel string) (string, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err.Error()
	}
	return filepath.Join(home, rel), ""
}

func fileStatus(path string) (bool, string) {
	if path == "" {
		return false, "path is empty"
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, ""
		}
		return false, err.Error()
	}
	if info.IsDir() {
		return false, "path is a directory"
	}
	return true, ""
}
