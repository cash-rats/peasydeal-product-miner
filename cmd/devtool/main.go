package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	runnerPkg "peasydeal-product-miner/internal/runner"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "chrome":
		if err := cmdChrome(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	case "doctor":
		if err := cmdDoctor(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	case "once":
		if err := cmdOnce(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `devtool: developer helper commands

Usage:
  go run ./cmd/devtool chrome [--port 9222] [--profile-dir <dir>]
  go run ./cmd/devtool doctor [--port 9222]
  go run ./cmd/devtool once --url <url> [--prompt-file <path>] [--out-dir <dir>]

Env vars:
  CHROME_DEBUG_PORT    default 9222
  CHROME_PROFILE_DIR   default $HOME/chrome-mcp-profiles/shopee
  CODEX_CMD            default "codex" (used by cmd/runner and devtool once)
`)
}

func cmdChrome(args []string) error {
	fs := flag.NewFlagSet("chrome", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	defPort := getenvDefault("CHROME_DEBUG_PORT", "9222")
	defProfile := getenvDefault("CHROME_PROFILE_DIR", filepath.Join(userHomeDir(), "chrome-mcp-profiles", "shopee"))
	port := fs.String("port", defPort, "Chrome DevTools remote debugging port")
	profileDir := fs.String("profile-dir", defProfile, "Dedicated Chrome profile directory (non-default)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*port) == "" {
		return errors.New("missing --port")
	}
	if strings.TrimSpace(*profileDir) == "" {
		return errors.New("missing --profile-dir")
	}

	if err := os.MkdirAll(*profileDir, 0o755); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS: use `open` so it starts as a normal app instance.
		cmd := exec.Command("open", "-na", "Google Chrome", "--args",
			"--remote-debugging-port="+*port,
			"--user-data-dir="+*profileDir,
		)
		return cmd.Start()
	case "linux":
		bin, err := findFirstInPath([]string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"})
		if err != nil {
			return err
		}
		cmd := exec.Command(bin,
			"--remote-debugging-port="+*port,
			"--user-data-dir="+*profileDir,
		)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		return cmd.Start()
	default:
		return fmt.Errorf("unsupported OS for auto-launch: %s (start Chrome manually with --remote-debugging-port and --user-data-dir)", runtime.GOOS)
	}
}

func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	defPort := getenvDefault("CHROME_DEBUG_PORT", "9222")
	port := fs.String("port", defPort, "Chrome DevTools remote debugging port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:%s/json/version", *port)
	fmt.Println("Checking:", url)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("Chrome DevTools not reachable (is Chrome running with --remote-debugging-port=%s?): %w", *port, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s from %s", resp.Status, url)
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*32))
	if len(bytesTrimSpace(b)) == 0 {
		return fmt.Errorf("empty response from %s", url)
	}
	fmt.Println("OK: Chrome DevTools reachable.")
	return nil
}

func cmdOnce(args []string) error {
	fs := flag.NewFlagSet("once", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	url := fs.String("url", "", "Shopee product URL")
	promptFile := fs.String("prompt-file", filepath.Join("config", "prompt.product.txt"), "Prompt template file path")
	outDir := fs.String("out-dir", "out", "Output directory for result JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*url) == "" {
		return errors.New("missing required flag: --url")
	}

	outPath, _, err := runnerPkg.RunOnce(runnerPkg.Options{
		URL:              *url,
		PromptFile:       *promptFile,
		OutDir:           *outDir,
		CodexCmd:         getenvDefault("CODEX_CMD", "codex"),
		SkipGitRepoCheck: getenvBool("CODEX_SKIP_GIT_REPO_CHECK", false),
	})
	if outPath != "" {
		fmt.Println(outPath)
	}
	// If we managed to write an output file, treat crawl errors as non-fatal.
	if err != nil && outPath == "" {
		return err
	}
	return nil
}

func getenvDefault(key string, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getenvBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

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

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
