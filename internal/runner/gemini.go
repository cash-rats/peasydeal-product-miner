package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

/*
 - [ ] Refactor with fx
 - [ ] Ensure result is in valid json format
*/

type GeminiRunnerConfig struct {
	Cmd     string
	Model   string
	WorkDir string
	Logger  *zap.SugaredLogger
}

type GeminiRunner struct {
	cmd     string
	model   string
	workDir string
	logger  *zap.SugaredLogger

	execCommand        func(name string, args ...string) *exec.Cmd
	execCommandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewGeminiRunner(cfg GeminiRunnerConfig) *GeminiRunner {
	return &GeminiRunner{
		cmd:                cfg.Cmd,
		model:              cfg.Model,
		workDir:            resolveRunnerWorkDir(cfg.WorkDir),
		execCommand:        exec.Command,
		execCommandContext: exec.CommandContext,
		logger:             cfg.Logger,
	}
}

func (r *GeminiRunner) Name() string { return "gemini" }

func (r *GeminiRunner) CheckAuth() error {
	paths := []string{
		".gemini/oauth_creds.json",
		".gemini/google_accounts.json",
	}
	for i, rel := range paths {
		path, pathErr := resolveHomePath(rel)
		if pathErr != "" {
			return fmt.Errorf("gemini auth file path error: %s", pathErr)
		}
		exists, errText := fileStatus(path)
		if errText != "" {
			return fmt.Errorf("gemini auth file error: %s", errText)
		}
		if exists {
			break
		}
		if i == len(paths)-1 {
			return fmt.Errorf("gemini auth file not found")
		}
	}

	ok, errText := r.runAuthProbe()
	if !ok {
		return fmt.Errorf("gemini auth probe failed: %s", errText)
	}
	return nil
}

func (r *GeminiRunner) Run(url string, prompt string) (string, error) {
	modelText, err := r.runModelText(url, prompt)
	if err != nil {
		return "", err
	}

	if _, err := extractJSONObjectWithStatus(modelText); err != nil {
		return "", fmt.Errorf("gemini returned non-JSON output: %w", err)
	}
	return modelText, nil
}

func (r *GeminiRunner) runModelText(url string, prompt string) (string, error) {
	// gemini [query..]
	// We use -o json to ensure we get parsable output.
	args := []string{"-o", "json"}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}

	r.logger.Infof("🏃🏻 running on model: %v", r.model)

	// Skills rely on tool calls (MCP DevTools + filesystem writes for artifacts). In CI/worker/headless runs
	// we must auto-approve tool actions. Allow overriding via env for safer deployments.
	args = append(args, "--approval-mode", "yolo")

	// Allow writing artifacts under mounted workspace paths in Docker/headless runs.
	// Gemini CLI errors when a listed directory does not exist, so only include existing dirs.
	for _, dir := range geminiIncludeDirectories(r.workDir) {
		args = append(args, "--include-directories", dir)
	}

	// Allow the MCP server used by our DevTools-based prompts. Without this, Gemini CLI may deny
	// MCP tool calls in non-interactive mode due to policy.
	args = append(args, "--allowed-mcp-server-names", "chrome-devtools")

	// Non-interactive (headless) mode is required for automation. Without -p/--prompt, Gemini CLI defaults to
	// interactive mode which behaves differently (and can block tool-use/skills when run non-interactively).
	args = append(args, "--prompt", prompt)

	start := time.Now()
	r.logger.Infow("crawl_started", "tool", "gemini", "url", url)

	cmd := r.execCommand(r.cmd, args...)
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		r.logger.Errorf(
			"⏱️ crawl failed tool=gemini url=%s duration=%s err=%s",
			url,
			time.Since(start).Round(time.Millisecond),
			fmt.Sprintf("std err %v, command err %v", cmd.Stderr, err.Error()),
		)
		return "", fmt.Errorf("gemini failed: %s", err.Error())
	}

	raw := stdout.String()
	r.logger.Infow(
		"crawl_finished",
		"tool", "gemini",
		"url", url,
		"raw_truncated", len(raw) > 8000,
		"raw_preview", previewText(raw, 8000),
		"duration", time.Since(start).Round(time.Millisecond).String(),
	)

	if unwrapped, ok := unwrapGeminiJSON(raw); ok {
		r.logGeminiOutput(url, unwrapped)
		return unwrapped, nil
	}
	r.logGeminiOutput(url, raw)
	return raw, nil
}

func (r *GeminiRunner) runAuthProbe() (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	args := []string{"-o", "json", "--prompt", "Return exactly: OK"}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	for _, dir := range geminiIncludeDirectories(r.workDir) {
		args = append(args, "--include-directories", dir)
	}

	cmd := r.execCommandContext(ctx, r.cmd, args...)
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, "timeout"
		}
		return false, "Seems like gemini is not authenticated"
	}
	return true, ""
}

// unwrapGeminiJSON extracts the tool's "response" field when Gemini is invoked with `-o json`.
// That output format is a wrapper object like:
//
//	{ "session_id": "...", "response": "<model text>", "stats": {...} }
//
// We want stdout to be the model text only (which should be the JSON contract).
func unwrapGeminiJSON(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return "", false
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", false
	}

	resp, ok := obj["response"]
	if !ok {
		return "", false
	}

	switch v := resp.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return "", false
		}
		return s, true
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return "", false
		}
		return s, true
	default:
		return "", false
	}
}

func (r *GeminiRunner) logGeminiOutput(url string, out string) {
	truncated := false
	if out == "" {
		r.logger.Debugw(
			"llm_output",
			"tool", "gemini",
			"url", url,
			"empty", true,
		)
		return
	}

	const maxChars = 8000
	if len(out) > maxChars {
		truncated = true
		out = out[:maxChars] + "...(truncated)"
	}
	r.logger.Debugw("llm_output", "tool", "gemini", "url", url, "truncated", truncated, "output", out)
}

func previewText(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func geminiIncludeDirectories(workDir string) []string {
	candidates := []string{
		"/out",
		"/app",
		"/gemini",
		"/codex",
	}
	if strings.TrimSpace(workDir) != "" {
		candidates = append(candidates,
			workDir,
			filepath.Join(workDir, "out"),
			filepath.Join(workDir, "gemini"),
			filepath.Join(workDir, "codex"),
		)
	}

	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, p := range candidates {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs := p
		if !filepath.IsAbs(abs) {
			base := workDir
			if strings.TrimSpace(base) == "" {
				base, _ = os.Getwd()
			}
			abs = filepath.Join(base, p)
		}
		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			continue
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		out = append(out, abs)
	}
	return out
}

var _ ToolRunner = (*GeminiRunner)(nil)
