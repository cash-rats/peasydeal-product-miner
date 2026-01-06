package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var allowedStatus = map[string]bool{
	"ok":           true,
	"needs_manual": true,
	"error":        true,
}

type Result map[string]any

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func loadPrompt(promptPath string, url string) (string, error) {
	b, err := os.ReadFile(promptPath)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(b), "{{URL}}", url), nil
}

func runCodex(codexCmd string, prompt string) (string, error) {
	cmd := exec.Command(codexCmd, "exec", prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("codex exec failed: %s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func validateMinimal(r Result) error {
	url, ok := r["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return fmt.Errorf("missing/invalid required key: url")
	}
	status, ok := r["status"].(string)
	if !ok || !allowedStatus[status] {
		return fmt.Errorf("missing/invalid required key: status")
	}
	capturedAt, ok := r["captured_at"].(string)
	if !ok || strings.TrimSpace(capturedAt) == "" {
		return fmt.Errorf("missing/invalid required key: captured_at")
	}
	return nil
}

func main() {
	var (
		url        = flag.String("url", "", "Shopee product URL")
		promptFile = flag.String("prompt-file", "", "Prompt template file path")
		outDir     = flag.String("out-dir", "", "Output directory for result JSON")
		codexCmd   = flag.String("codex-cmd", getenvDefault("CODEX_CMD", "codex"), "Codex CLI command name/path")
	)
	flag.Parse()

	if strings.TrimSpace(*url) == "" || strings.TrimSpace(*promptFile) == "" || strings.TrimSpace(*outDir) == "" {
		fmt.Fprintln(os.Stderr, "missing required flags: --url, --prompt-file, --out-dir")
		os.Exit(2)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	prompt, err := loadPrompt(*promptFile, *url)
	if err != nil {
		writeError(*outDir, *url, err)
		return
	}

	raw, err := runCodex(*codexCmd, prompt)
	if err != nil {
		writeError(*outDir, *url, err)
		return
	}

	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		writeError(*outDir, *url, fmt.Errorf("invalid JSON from codex: %w", err))
		return
	}

	obj, ok := parsed.(map[string]any)
	if !ok {
		writeError(*outDir, *url, fmt.Errorf("output JSON is not an object"))
		return
	}

	r := Result(obj)
	if _, ok := r["url"]; !ok {
		r["url"] = *url
	}
	if _, ok := r["captured_at"]; !ok {
		r["captured_at"] = nowISO()
	}
	if err := validateMinimal(r); err != nil {
		writeError(*outDir, *url, err)
		return
	}

	if err := writeResult(*outDir, r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeError(outDir string, url string, err error) {
	r := Result{
		"url":         url,
		"status":      "error",
		"captured_at": nowISO(),
		"error":       err.Error(),
	}
	if werr := writeResult(outDir, r); werr != nil {
		fmt.Fprintln(os.Stderr, werr)
	}
}

func writeResult(outDir string, r Result) error {
	ts := time.Now().UTC().Format("20060102T150405Z")
	outPath := filepath.Join(outDir, ts+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(outPath, b, 0o644); err != nil {
		return err
	}
	fmt.Println(outPath)
	return nil
}

func getenvDefault(key string, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

