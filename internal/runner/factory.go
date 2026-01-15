package runner

import (
	"fmt"
	"strings"
)

func NewToolRunnerFromOptions(opts Options) (ToolRunner, error) {
	tool := strings.TrimSpace(opts.Tool)
	if tool == "" {
		tool = "codex"
	}

	cmd := strings.TrimSpace(opts.Cmd)
	if cmd == "" {
		cmd = strings.TrimSpace(opts.CodexCmd)
	}

	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = strings.TrimSpace(opts.CodexModel)
	}

	switch tool {
	case "codex":
		if cmd == "" {
			cmd = "codex"
		}
		return NewCodexRunner(CodexRunnerConfig{
			Cmd:              cmd,
			Model:            model,
			SkipGitRepoCheck: opts.SkipGitRepoCheck,
		}), nil
	case "gemini":
		if cmd == "" {
			cmd = "gemini"
		}
		return NewGeminiRunner(GeminiRunnerConfig{
			Cmd:   cmd,
			Model: model,
		}), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}
