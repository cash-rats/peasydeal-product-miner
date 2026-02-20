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
	RunID      string // optional run id for artifact correlation; injected into skill-mode prompt when non-empty

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
	opts.RunID = strings.TrimSpace(opts.RunID)
	if opts.RunID == "" {
		opts.RunID = strings.TrimSpace(os.Getenv("CRAWL_RUN_ID"))
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

	normalizeVariationImages(res)
}

func normalizeVariationImages(res Result) {
	raw, ok := res["variations"]
	if !ok || raw == nil {
		return
	}

	vars, ok := raw.([]any)
	if !ok {
		return
	}

	for i, item := range vars {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}

		images := collectVariationImages(obj)
		obj["images"] = images
		delete(obj, "image")
		vars[i] = obj
	}

	res["variations"] = vars
}

func collectVariationImages(obj map[string]any) []string {
	out := make([]string, 0, 4)
	seen := make(map[string]bool, 4)

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	if raw, ok := obj["images"]; ok && raw != nil {
		switch vv := raw.(type) {
		case []any:
			for _, it := range vv {
				if s, ok := it.(string); ok {
					add(s)
				}
			}
		case []string:
			for _, s := range vv {
				add(s)
			}
		case string:
			add(vv)
		}
	}

	if v, ok := obj["image"].(string); ok {
		add(v)
	}

	return out
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
		return "", res, err
	}

	if opts.PromptMode == promptModeLegacy && opts.PromptFile == "" {
		c, err := crawler.ForSource(src)
		if err != nil {
			res := errorResult(opts.URL, err)
			return "", res, err
		}
		opts.PromptFile = c.DefaultPromptFile()
	}

	prompt, err := buildPrompt(opts, src)
	r.logger.Infof("📨 prompt used: %v", prompt)
	if err != nil {
		res := errorResult(opts.URL, err)
		return "", res, err
	}

	tr, ok := r.runners[opts.Tool]
	if !ok {
		err := fmt.Errorf("❌ Unknown tool: %s", opts.Tool)
		res := errorResult(opts.URL, err)
		return "", res, err
	}

	authErr := tr.CheckAuth()
	raw, runErr := tr.Run(opts.URL, prompt)
	var res Result
	outPath := ""
	if isOrchestratorSkillMode(opts, src) {
		outPath = orchestratorFinalPath(opts)
		r.logger.Infow(
			"runner_orchestrator_artifact_final_read",
			"tool", tr.Name(),
			"url", opts.URL,
			"path", outPath,
		)
		res, err = loadOrchestratorFinalResult(opts, src)
		if err != nil && runErr != nil {
			err = fmt.Errorf("%w (tool error: %v)", err, runErr)
		}
		if err != nil {
			res = errorResult(opts.URL, err)
			if authErr != nil {
				res["auth_check_error"] = authErr.Error()
			}
			return outPath, res, err
		}
	} else {
		if runErr != nil {
			err := runErr
			res := errorResult(opts.URL, err)
			if authErr != nil {
				res["auth_check_error"] = authErr.Error()
			}
			return outPath, res, err
		}

		var err error
		res, _, err = parseResult(tr.Name(), raw)
		if err != nil {
			res = errorResult(opts.URL, err)
			if authErr != nil {
				res["auth_check_error"] = authErr.Error()
			}
			return outPath, res, err
		}
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
		return outPath, res, verr
	}

	return outPath, res, nil
}

func loadOrchestratorFinalResult(opts Options, src source.Source) (Result, error) {
	if opts.PromptMode != promptModeSkill {
		return nil, fmt.Errorf("orchestrator fallback disabled: prompt mode is not skill")
	}

	skillName := strings.TrimSpace(opts.SkillName)
	if skillName == "" {
		skillName = defaultSkillName(src)
	}
	if !isSupportedOrchestratorSkill(skillName) {
		return nil, fmt.Errorf(
			"orchestrator fallback disabled: unsupported skill %q (allowed: %s, %s)",
			skillName,
			shopeeOrchestratorPipelineSkill,
			taobaoOrchestratorPipelineSkill,
		)
	}
	if strings.TrimSpace(opts.RunID) == "" {
		return nil, fmt.Errorf("orchestrator final artifact requires non-empty run id")
	}

	finalPath := orchestratorFinalPath(opts)
	b, err := os.ReadFile(finalPath)
	if err != nil {
		return nil, fmt.Errorf("read orchestrator final artifact: %w (path=%s)", err, finalPath)
	}

	res, _, err := parseResult("orchestrator-final-artifact", string(b))
	if err != nil {
		return nil, fmt.Errorf("parse orchestrator final artifact: %w", err)
	}
	if status, _ := res["status"].(string); strings.TrimSpace(status) == "error" {
		msg, _ := res["error"].(string)
		msg = strings.TrimSpace(msg)
		if msg == "" {
			msg = "final artifact status is error"
		}
		return nil, fmt.Errorf("orchestrator final artifact status error: %s", msg)
	}
	res["result_source"] = "artifact_final"
	res["artifact_final_path"] = finalPath
	return res, nil
}

func orchestratorFinalPath(opts Options) string {
	return filepath.Join(opts.OutDir, "artifacts", opts.RunID, "final.json")
}

func isOrchestratorSkillMode(opts Options, src source.Source) bool {
	if opts.PromptMode != promptModeSkill {
		return false
	}
	skillName := strings.TrimSpace(opts.SkillName)
	if skillName == "" {
		skillName = defaultSkillName(src)
	}
	return isSupportedOrchestratorSkill(skillName)
}

func isSupportedOrchestratorSkill(skillName string) bool {
	switch strings.TrimSpace(skillName) {
	case shopeeOrchestratorPipelineSkill, taobaoOrchestratorPipelineSkill:
		return true
	default:
		return false
	}
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
		return buildSkillPrompt(src, opts.URL, opts.SkillName, opts.Tool, opts.RunID, opts.OutDir)
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
