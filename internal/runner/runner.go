package runner

import (
	"bytes"
	"encoding/json"
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

type Options struct {
	URL        string
	PromptFile string
	OutDir     string
	CodexCmd   string
}

func RunOnce(opts Options) (string, Result, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return "", nil, fmt.Errorf("missing URL")
	}
	if strings.TrimSpace(opts.PromptFile) == "" {
		return "", nil, fmt.Errorf("missing PromptFile")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return "", nil, fmt.Errorf("missing OutDir")
	}
	if strings.TrimSpace(opts.CodexCmd) == "" {
		opts.CodexCmd = "codex"
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", nil, err
	}

	prompt, err := loadPrompt(opts.PromptFile, opts.URL)
	if err != nil {
		r := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	raw, err := runCodex(opts.CodexCmd, prompt)
	if err != nil {
		r := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	r, err := parseResult(raw)
	if err != nil {
		r = errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	r.setdefault("url", opts.URL)
	r.setdefault("captured_at", nowISO())
	if verr := validateMinimal(r); verr != nil {
		r = errorResult(opts.URL, verr)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, verr
	}

	outPath, err := writeResult(opts.OutDir, r)
	return outPath, r, err
}

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

func parseResult(raw string) (Result, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON from codex: %w", err)
	}
	obj, ok := parsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("output JSON is not an object")
	}
	return Result(obj), nil
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

func errorResult(url string, err error) Result {
	return Result{
		"url":         url,
		"status":      "error",
		"captured_at": nowISO(),
		"error":       err.Error(),
	}
}

func writeResult(outDir string, r Result) (string, error) {
	ts := time.Now().UTC().Format("20060102T150405Z")
	outPath := filepath.Join(outDir, ts+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	b = append(b, '\n')
	if err := os.WriteFile(outPath, b, 0o644); err != nil {
		return "", err
	}
	return outPath, nil
}

// setdefault mimics Python's dict.setdefault.
func (r Result) setdefault(key string, value any) {
	if _, ok := r[key]; !ok {
		r[key] = value
	}
}

