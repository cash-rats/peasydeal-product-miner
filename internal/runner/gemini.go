package runner

import (
	"bytes"
	"context"
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

	if _, err := extractJSONObjectWithStatus(modelText); err == nil {
		return modelText, nil
	}

	// If Gemini cut off the JSON mid-output (common when output is too long or the model hits limits),
	// a "repair" prompt can't recover missing fields. First try re-running the original crawl prompt,
	// but with explicit output size limits to avoid truncation.
	if diagnoseContractIssue(modelText) == "invalid or truncated JSON" {
		r.logger.Infow(
			"runner_gemini_retry_truncation",
			"tool", "gemini",
			"url", url,
		)

		retryPrompt := buildGeminiTruncationRetryPrompt(prompt)
		retriedText, rerr := r.runModelText(url, retryPrompt)
		if rerr == nil {
			if _, perr := extractJSONObjectWithStatus(retriedText); perr == nil {
				r.logger.Infow("runner_gemini_retry_truncation_succeeded", "tool", "gemini", "url", url)
				return retriedText, nil
			}
		}
	}

	r.logger.Infow(
		"runner_gemini_repair_attempt",
		"tool", "gemini",
		"url", url,
		"err", diagnoseContractIssue(modelText),
	)
	repairPrompt := buildGeminiRepairPrompt(url, modelText)

	repairedText, rerr := r.runModelText(url, repairPrompt)
	if rerr != nil {
		r.logger.Infow("runner_gemini_repair_failed", "tool", "gemini", "url", url, "err", rerr.Error())
		return "", fmt.Errorf("gemini returned non-JSON output: %w", rerr)
	}

	if _, perr := extractJSONObjectWithStatus(repairedText); perr != nil {
		r.logger.Infow("runner_gemini_repair_failed", "tool", "gemini", "url", url, "err", perr.Error())
		return "", fmt.Errorf("gemini returned non-JSON output: %w", perr)
	}

	r.logger.Infow("runner_gemini_repair_succeeded", "tool", "gemini", "url", url)
	return repairedText, nil
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
  "price": "number or numeric string",
  "images": ["string"] (optional; empty array allowed),
  "variations": [
    {
      "title": "string",
      "position": "int",
      "image": "string (optional)"
    }
  ]
}

Rules:
- Output JSON ONLY. No markdown fences. No extra text.
- Do not call any tools.
- url must be %q.
- If required fields are missing, set status="error" and explain in error.
- If the text indicates a login/verification/CAPTCHA wall, set status="needs_manual" and explain in notes.
- Always include "variations": [] if you can't infer variations.
- If images are too many, include only the first 20.
- If description is too long, truncate it to 1500 characters.

TEXT:
<<<
%s
>>>
`, url, previousOutput)
}

func buildGeminiTruncationRetryPrompt(originalPrompt string) string {
	// IMPORTANT: This string is appended to the crawl prompt that DOES use tools.
	// Keep it short so it doesn't bloat the tool-augmented context.
	return originalPrompt + `

Output limits (mandatory):
- Output MUST be exactly ONE JSON object and NOTHING ELSE.
- Always include "variations": [] when no variations found.
- If there are many images/variations, include only the first 20 of each.
- Truncate "description" to at most 1500 characters.
- Keep the whole JSON under ~6000 characters; if needed, drop optional fields (images/variations) first.
`
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

var _ ToolRunner = (*GeminiRunner)(nil)
