package runner

import (
	"encoding/json"
	"fmt"
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

// setdefault mimics Python's dict.setdefault.
func (r Result) setdefault(key string, value any) {
	if _, ok := r[key]; !ok {
		r[key] = value
	}
}

func (r Result) ensureImagesArray() {
	if v, ok := r["images"]; !ok || v == nil {
		r["images"] = []any{}
	}
}

type Options struct {
	URL        string `validate:"required"`
	PromptFile string
	OutDir     string `validate:"required"`
	Tool       string // "codex" or "gemini"
	PromptMode string `validate:"omitempty,oneof=legacy skill"` // "legacy" (default) or "skill"
	SkillName  string // optional override; used when PromptMode=skill

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

func normalizeOptions(opts Options) Options {
	opts.URL = strings.TrimSpace(opts.URL)
	opts.PromptFile = strings.TrimSpace(opts.PromptFile)
	opts.OutDir = strings.TrimSpace(opts.OutDir)
	opts.Tool = strings.TrimSpace(opts.Tool)
	opts.PromptMode = normalizePromptMode(opts.PromptMode)
	if opts.PromptMode == "" {
		opts.PromptMode = normalizePromptMode(os.Getenv("CRAWL_PROMPT_MODE"))
	}
	if opts.PromptMode == "" {
		opts.PromptMode = promptModeLegacy
	}
	opts.SkillName = strings.TrimSpace(opts.SkillName)
	if opts.SkillName == "" {
		opts.SkillName = strings.TrimSpace(os.Getenv("CRAWL_SKILL_NAME"))
	}
	if opts.Tool == "" {
		opts.Tool = "codex"
	}
	return opts
}

func normalizeResult(res Result) {
	if v, ok := res["currency"].(string); ok {
		clean := strings.TrimSpace(v)
		if clean == "" {
			delete(res, "currency")
		} else {
			res["currency"] = strings.ToUpper(clean)
		}
	}

	if v, ok := res["price"].(string); ok {
		if strings.TrimSpace(v) == "" {
			delete(res, "price")
		}
	}
}

func (r *Runner) RunOnce(opts Options) (string, Result, error) {
	opts = normalizeOptions(opts)
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

	if opts.PromptMode == promptModeLegacy && opts.PromptFile == "" {
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

	prompt, err := buildPrompt(opts, src)
	if err != nil {
		res := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	tr, ok := r.runners[opts.Tool]
	if !ok {
		err := fmt.Errorf("❌ Unknown tool: %s", opts.Tool)
		res := errorResult(opts.URL, err)
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	authErr := tr.CheckAuth()
	raw, err := tr.Run(opts.URL, prompt)
	if err != nil {
		res := errorResult(opts.URL, err)
		if authErr != nil {
			res["auth_check_error"] = authErr.Error()
		}
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}

	res, usedFallback, err := parseResult(tr.Name(), raw)
	if err != nil {
		res = errorResult(opts.URL, err)
		if authErr != nil {
			res["auth_check_error"] = authErr.Error()
		}
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, err
	}
	if usedFallback {
		r.logger.Debugw(
			"runner_parse_result_fallback",
			"tool", tr.Name(),
			"url", opts.URL,
		)
	}

	res.setdefault("url", opts.URL)
	res.setdefault("source", string(src))
	res.setdefault("captured_at", nowISO())
	res.ensureImagesArray()
	normalizeResult(res)
	if authErr != nil {
		res["auth_check_error"] = authErr.Error()
	}
	if verr := validateContract(res); verr != nil {
		res = errorResult(opts.URL, verr)
		if authErr != nil {
			res["auth_check_error"] = authErr.Error()
		}
		outPath, werr := writeResult(opts.OutDir, res)
		if werr != nil {
			return "", res, werr
		}
		return outPath, res, verr
	}

	outPath, err := writeResult(opts.OutDir, res)
	return outPath, res, err
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

func buildPrompt(opts Options, src source.Source) (string, error) {
	switch opts.PromptMode {
	case promptModeSkill:
		if opts.PromptFile != "" {
			return "", fmt.Errorf("prompt_file is not supported when prompt_mode=skill")
		}
		return buildSkillPrompt(src, opts.URL, opts.SkillName)
	case promptModeLegacy:
		return loadPrompt(opts.PromptFile, opts.URL)
	default:
		return "", fmt.Errorf("unsupported prompt_mode: %q", opts.PromptMode)
	}
}

func parseResult(toolName string, raw string) (Result, bool, error) {
	extracted, err := extractJSONObjectWithStatus(raw)
	if err != nil {
		extracted, err = extractFirstJSONObject(raw)
		if err != nil {
			if strings.TrimSpace(toolName) == "" {
				toolName = "tool"
			}
			return nil, false, fmt.Errorf("invalid JSON from %s: %w", toolName, err)
		}
		res, derr := parseResultDecoded(toolName, extracted)
		return res, true, derr
	}
	res, derr := parseResultDecoded(toolName, extracted)
	return res, false, derr
}

func parseResultDecoded(toolName string, extracted string) (Result, error) {
	dec := json.NewDecoder(strings.NewReader(extracted))
	dec.UseNumber()

	var parsed any
	if err := dec.Decode(&parsed); err != nil {
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

func validateContract(r Result) error {
	// Validate only the prompt-defined output contract; ignore other keys that the runner may add.
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()

	var out CrawlOut
	if err := dec.Decode(&out); err != nil {
		return fmt.Errorf("invalid contract JSON: %w", err)
	}
	out.Status = strings.TrimSpace(out.Status)
	if !allowedStatus[out.Status] {
		return fmt.Errorf("missing/invalid required key: status")
	}
	return validateCrawlOut(out)
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
