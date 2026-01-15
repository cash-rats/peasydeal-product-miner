package runner

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type CodexRunnerConfig struct {
	Cmd              string
	Model            string
	SkipGitRepoCheck bool
}

type CodexRunner struct {
	cmd              string
	model            string
	skipGitRepoCheck bool

	execCommand func(name string, args ...string) *exec.Cmd
}

func NewCodexRunner(cfg CodexRunnerConfig) *CodexRunner {
	return &CodexRunner{
		cmd:              cfg.Cmd,
		model:            cfg.Model,
		skipGitRepoCheck: cfg.SkipGitRepoCheck,
		execCommand:      exec.Command,
	}
}

func (r *CodexRunner) Name() string { return "codex" }

func (r *CodexRunner) Run(url string, prompt string) (string, error) {
	// Codex CLI expects exec-scoped flags after the subcommand:
	//   codex exec --skip-git-repo-check "<prompt>"
	args := []string{"exec"}
	if r.skipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	if strings.TrimSpace(r.model) != "" {
		args = append(args, "--model", r.model)
	}
	args = append(args, prompt)

	start := time.Now()
	if strings.TrimSpace(r.model) != "" {
		log.Printf("⏱️ crawl started tool=codex url=%s model=%s", url, r.model)
	} else {
		log.Printf("⏱️ crawl started tool=codex url=%s", url)
	}

	cmd := r.execCommand(r.cmd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		log.Printf("⏱️ crawl failed tool=codex url=%s duration=%s err=%s", url, time.Since(start).Round(time.Millisecond), msg)
		return "", fmt.Errorf("codex exec failed: %s", msg)
	}

	log.Printf("⏱️ crawl finished tool=codex url=%s duration=%s", url, time.Since(start).Round(time.Millisecond))
	return strings.TrimSpace(stdout.String()), nil
}
