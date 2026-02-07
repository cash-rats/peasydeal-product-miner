package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

type CodexRunnerConfig struct {
	Cmd              string
	Model            string
	SkipGitRepoCheck bool
	WorkDir          string
	Logger           *zap.SugaredLogger
}

type CodexRunner struct {
	cmd              string
	model            string
	skipGitRepoCheck bool
	workDir          string
	logger           *zap.SugaredLogger

	execCommand        func(name string, args ...string) *exec.Cmd
	execCommandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

func NewCodexRunner(cfg CodexRunnerConfig) *CodexRunner {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &CodexRunner{
		cmd:                cfg.Cmd,
		model:              cfg.Model,
		skipGitRepoCheck:   cfg.SkipGitRepoCheck,
		workDir:            resolveRunnerWorkDir(cfg.WorkDir),
		logger:             logger,
		execCommand:        exec.Command,
		execCommandContext: exec.CommandContext,
	}
}

func (r *CodexRunner) Name() string { return "codex" }

func (r *CodexRunner) CheckAuth() error {
	path, pathErr := resolveHomePath(".codex/auth.json")
	if pathErr != "" {
		return fmt.Errorf("codex auth file path error: %s", pathErr)
	}

	exists, errText := fileStatus(path)
	if errText != "" {
		return fmt.Errorf("codex auth file error: %s", errText)
	}
	if !exists {
		return fmt.Errorf("codex auth file not found")
	}

	ok, probeErr := r.runAuthProbe()
	if !ok {
		return fmt.Errorf("codex auth probe failed: %s", probeErr)
	}
	return nil
}

func (r *CodexRunner) Run(url string, prompt string) (string, error) {
	modelText, err := r.runModelText(url, prompt)
	if err != nil {
		return "", err
	}

	if _, err := extractJSONObjectWithStatus(modelText); err == nil {
		r.logCodexOutput(url, modelText)
		return modelText, nil
	}

	if _, err := extractFirstJSONObject(modelText); err == nil {
		r.logger.Infow(
			"runner_codex_repair_attempt",
			"tool", "codex",
			"url", url,
			"err", "missing status",
		)
	} else {
		r.logger.Infow(
			"runner_codex_repair_attempt",
			"tool", "codex",
			"url", url,
			"err", err.Error(),
		)
	}
	repairPrompt := buildCodexRepairPrompt(url, modelText)

	repairedText, rerr := r.runModelText(url, repairPrompt)
	if rerr != nil {
		r.logger.Infow(
			"runner_codex_repair_failed",
			"tool", "codex",
			"url", url,
			"err", rerr.Error(),
		)
		return "", fmt.Errorf("codex returned non-JSON output: %w", rerr)
	}

	if _, perr := extractJSONObjectWithStatus(repairedText); perr != nil {
		r.logger.Infow(
			"runner_codex_repair_failed",
			"tool", "codex",
			"url", url,
			"err", perr.Error(),
		)
		return "", fmt.Errorf("codex returned non-JSON output: %w", perr)
	}

	r.logger.Infow(
		"runner_codex_repair_succeeded",
		"tool", "codex",
		"url", url,
	)
	r.logCodexOutput(url, repairedText)
	return repairedText, nil
}

func (r *CodexRunner) runModelText(url string, prompt string) (string, error) {
	// Codex CLI expects exec-scoped flags after the subcommand:
	//   codex exec --skip-git-repo-check --model <model> "<prompt>"
	args := []string{"exec"}
	if r.skipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}

	r.logger.Infof("🏃🏻 running on model: %v", r.model)

	args = append(args, prompt)
	start := time.Now()
	r.logger.Infow(
		"crawl_started",
		"tool", "codex",
		"url", url,
		"model", r.model,
	)

	cmd := r.execCommand(r.cmd, args...)
	if r.workDir != "" {
		cmd.Dir = r.workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		r.logger.Infow(
			"crawl_failed",
			"tool", "codex",
			"url", url,
			"duration", time.Since(start).Round(time.Millisecond).String(),
			"err", err.Error(),
		)
		return "", fmt.Errorf("codex exec failed: %s", err.Error())
	}

	r.logger.Infow(
		"crawl_finished",
		"tool", "codex",
		"url", url,
		"duration", time.Since(start).Round(time.Millisecond).String(),
	)

	return stdout.String(), nil
}

func (r *CodexRunner) runAuthProbe() (bool, string) {
	if strings.TrimSpace(r.cmd) == "" {
		return false, "missing codex command"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	args := []string{"exec"}
	if r.skipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	if r.model != "" {
		args = append(args, "--model", r.model)
	}
	args = append(args, "Return exactly: OK")

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
		return false, formatCodexAuthErr()
	}
	return true, ""
}

func formatCodexAuthErr() string {
	return "Seems like codex is not authenticated"
}

func (r *CodexRunner) logCodexOutput(url string, out string) {
	truncated := false
	if out == "" {
		r.logger.Debugw(
			"llm_output",
			"tool", "codex",
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

	r.logger.Debugw("llm_output", "tool", "codex", "url", url, "truncated", truncated, "output", out)
}

func buildCodexRepairPrompt(url, previousOutput string) string {
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

TEXT:
<<<
%s
>>>
`, url, previousOutput)
}

var _ ToolRunner = (*CodexRunner)(nil)
