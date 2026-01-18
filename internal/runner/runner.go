package runner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"peasydeal-product-miner/internal/crawler"
	"peasydeal-product-miner/internal/source"

	"github.com/go-playground/validator/v10"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var allowedStatus = map[string]bool{
	"ok":           true,
	"needs_manual": true,
	"error":        true,
}

type Runner struct {
	logger    *zap.SugaredLogger
	runners   map[string]ToolRunner
	validator *validator.Validate
}

type NewRunnerParams struct {
	fx.In

	Runners map[string]ToolRunner
	Logger  *zap.SugaredLogger
}

func NewRunner(p NewRunnerParams) *Runner {
	return &Runner{
		runners:   p.Runners,
		logger:    p.Logger,
		validator: validator.New(),
	}
}

type Result map[string]any

type Options struct {
	URL        string `validate:"required"`
	PromptFile string
	OutDir     string `validate:"required"`
	Tool       string // "codex" or "gemini"

	// Cmd is the binary name/path to execute (e.g. "codex" or "gemini").
	// If empty, CodexCmd is used for backward compatibility.
	Cmd string

	// Model passes `--model` to tools that support it (Codex CLI and Gemini CLI).
	// If empty, CodexModel is used for backward compatibility.
	Model string

	// CodexCmd is a deprecated alias for Cmd.
	CodexCmd string

	// CodexModel is a deprecated alias for Model.
	// It historically passed `--model` to Codex CLI when non-empty.
	CodexModel string
	// SkipGitRepoCheck passes `--skip-git-repo-check` to Codex CLI.
	// This is useful in containers or non-git directories.
	SkipGitRepoCheck bool
}

func (r *Runner) RunOnce(opts Options) (string, Result, error) {
	if err := r.validator.Struct(opts); err != nil {
		r.logger.Errorf("❌ Missing required field value %v", err)

		return "", nil, fmt.Errorf("missing OutDir %v", err)
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", nil, err
	}

	src, err := source.Detect(opts.URL)
	if err != nil {
		res := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	if opts.PromptFile == "" {
		c, err := crawler.ForSource(src)
		if err != nil {
			res := errorResult(opts.URL, err)
			outPath, werr := writeResult(opts.OutDir, res)
			if werr != nil {
				return "", res, werr
			}
			return outPath, res, err
		}
		opts.PromptFile = c.DefaultPromptFile()
	}

	prompt, err := loadPrompt(opts.PromptFile, opts.URL)
	if err != nil {
		res := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	tool := opts.Tool
	if tool == "" {
		tool = "codex"
	}
	tr, ok := r.runners[tool]
	if !ok {
		err := fmt.Errorf("❌ Unknown tool: %s", tool)
		res := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	raw, err := tr.Run(opts.URL, prompt)
	if err != nil {
		res := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	res, err := parseResult(tr.Name(), raw)
	if err != nil {
		res = errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	res.setdefault("url", opts.URL)
	res.setdefault("source", string(src))
	res.setdefault("captured_at", nowISO())
	if verr := validateMinimal(res); verr != nil {
		res = errorResult(opts.URL, verr)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, verr
	}

	outPath, err := writeResult(opts.OutDir, res)
	return outPath, res, err
}

func RunOnce(opts Options) (string, Result, error) {
	return runOnce(opts, NewToolRunnerFromOptions)
}

func runOnce(opts Options, toolRunnerFromOptions func(Options) (ToolRunner, error)) (string, Result, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return "", nil, fmt.Errorf("missing URL")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return "", nil, fmt.Errorf("missing OutDir")
	}

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return "", nil, err
	}

	src, err := source.Detect(opts.URL)
	if err != nil {
		r := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	if strings.TrimSpace(opts.PromptFile) == "" {
		c, err := crawler.ForSource(src)
		if err != nil {
			r := errorResult(opts.URL, err)
			outPath, werr := writeResult(opts.OutDir, r)
			if werr != nil {
				return "", r, werr
			}
			return outPath, r, err
		}
		opts.PromptFile = c.DefaultPromptFile()
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

	tr, err := toolRunnerFromOptions(opts)
	if err != nil {
		r := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	log.Printf("📄 prompt selected url=%s source=%s prompt_file=%s tool=%s", opts.URL, src, opts.PromptFile, tr.Name())

	raw, err := tr.Run(opts.URL, prompt)
	if err != nil {
		r := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	r, err := parseResult(tr.Name(), raw)
	if err != nil {
		r = errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, r)
		if werr != nil {
			return "", r, werr
		}
		return outPath, r, err
	}

	r.setdefault("url", opts.URL)
	r.setdefault("source", string(src))
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

func parseResult(toolName string, raw string) (Result, error) {
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		if strings.TrimSpace(toolName) == "" {
			toolName = "tool"
		}
		return nil, fmt.Errorf("invalid JSON from %s: %w", toolName, err)
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
	outPath := filepath.Join(outDir, ts+"_"+resultSource(r)+".json")
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

func resultSource(r Result) string {
	rawURL, _ := r["url"].(string)

	if detected, err := source.Detect(rawURL); err == nil && detected != "" {
		return string(detected)
	}
	return "unknown"
}

// setdefault mimics Python's dict.setdefault.
func (r Result) setdefault(key string, value any) {
	if _, ok := r[key]; !ok {
		r[key] = value
	}
}
