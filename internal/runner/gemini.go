package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

/*
 - [ ] Refactor with fx
 - [ ] Ensure result is in valid json format
*/

type GeminiRunnerConfig struct {
	Cmd    string
	Model  string
	Logger *zap.SugaredLogger
}

type GeminiRunner struct {
	cmd    string
	model  string
	logger *zap.SugaredLogger

	execCommand func(name string, args ...string) *exec.Cmd
}

func NewGeminiRunner(cfg GeminiRunnerConfig) *GeminiRunner {
	return &GeminiRunner{
		cmd:         cfg.Cmd,
		model:       cfg.Model,
		execCommand: exec.Command,
		logger:      cfg.Logger,
	}
}

func (r *GeminiRunner) Name() string { return "gemini" }

func (r *GeminiRunner) Run(url string, prompt string) (string, error) {
	modelText, err := r.runModelText(url, prompt)
	if err != nil {
		return "", err
	}

	extracted, err := extractFirstJSONObject(modelText)
	if err == nil {
		r.logGeminiOutput(url, modelText)
		return extracted, nil
	}

	r.logger.Infow("runner_gemini_repair_attempt", "tool", "gemini", "url", url, "err", err.Error())
	repairPrompt := buildGeminiRepairPrompt(url, modelText)

	repairedText, rerr := r.runModelText(url, repairPrompt)
	if rerr != nil {
		r.logger.Infow("runner_gemini_repair_failed", "tool", "gemini", "url", url, "err", rerr.Error())
		return "", fmt.Errorf("gemini returned non-JSON output: %w", err)
	}

	repairedExtracted, perr := extractFirstJSONObject(repairedText)
	if perr != nil {
		r.logger.Infow("runner_gemini_repair_failed", "tool", "gemini", "url", url, "err", perr.Error())
		return "", fmt.Errorf("gemini returned non-JSON output: %w", err)
	}

	r.logger.Infow("runner_gemini_repair_succeeded", "tool", "gemini", "url", url)
	r.logGeminiOutput(url, repairedText)
	return repairedExtracted, nil
}

func (r *GeminiRunner) runModelText(url string, prompt string) (string, error) {
	// gemini [query..]
	// We use -o json to ensure we get parsable output.
	args := []string{"-o", "json"}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}

	// Allow the MCP server used by our DevTools-based prompts. Without this, Gemini CLI may deny
	// MCP tool calls in non-interactive mode due to policy.
	args = append(args, "--allowed-mcp-server-names", "chrome-devtools")
	args = append(args, prompt)

	start := time.Now()
	r.logger.Infow("crawl_started", "tool", "gemini", "url", url)

	cmd := r.execCommand(r.cmd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		r.logger.Errorf(
			"⏱️ crawl failed tool=gemini url=%s duration=%s err=%s",
			url,
			time.Since(start).Round(time.Millisecond),
			err.Error(),
		)
		return "", fmt.Errorf("gemini failed: %s", err.Error())
	}

	r.logger.Infow(
		"crawl_finished",
		"tool", "gemini",
		"url", url,
		"duration", time.Since(start).Round(time.Millisecond).String(),
	)

	raw := stdout.String()
	if unwrapped, ok := unwrapGeminiJSON(raw); ok {
		return strings.TrimSpace(unwrapped), nil
	}
	r.logGeminiOutput(url, raw)
	return strings.TrimSpace(raw), nil
}

func (r *GeminiRunner) logGeminiOutput(url string, out string) {
	out = strings.TrimSpace(out)
	truncated := false
	if out == "" {
		r.logger.Debugw("llm_output", "tool", "gemini", "url", url, "empty", true)
		return
	}

	const maxChars = 8000
	if len(out) > maxChars {
		truncated = true
		out = out[:maxChars] + "...(truncated)"
	}

	r.logger.Debugw("llm_output", "tool", "gemini", "url", url, "truncated", truncated, "output", out)
}

// unwrapGeminiJSON extracts the tool's "response" field when Gemini is invoked with `-o json`.
// That output format is a wrapper object like:
//
//	{ "session_id": "...", "response": "<model text>", "stats": {...} }
//
// We want stdout to be the model text only (which should be the JSON contract).
func unwrapGeminiJSON(raw string) (string, bool) {
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

func buildGeminiRepairPrompt(url, previousOutput string) string {
	if previousOutput == "" {
		previousOutput = "<empty>"
	}

	return fmt.Sprintf(`
You returned invalid JSON or did not follow the output contract.

Convert the TEXT below into EXACTLY ONE valid JSON object matching this contract:
{
  "url": "string",
  "status": "ok | needs_manual | error",
  "captured_at": "ISO-8601 UTC timestamp",
  "notes": "string (required when status=needs_manual)",
  "error": "string (required when status=error)",
  "title": "string",
  "description": "string",
  "currency": "string (e.g. TWD)",
  "price": "number or numeric string"
}

Rules:
- Output JSON ONLY. No markdown fences. No extra text.
- Do not call any tools.
- url must be %q.
- If required fields are missing, set status="error" and explain in error.
- If the text indicates a login/verification/CAPTCHA wall, set status="needs_manual" and explain in notes.

TEXT:
<<<
%s
>>>
`, url, previousOutput)
}

var _ ToolRunner = (*GeminiRunner)(nil)
