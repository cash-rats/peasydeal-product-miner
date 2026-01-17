package runner

import (
	"fmt"
	"strings"
)

type Service struct {
	runners map[string]ToolRunner
}

func NewService(runners map[string]ToolRunner) *Service {
	return &Service{runners: runners}
}

func (s *Service) RunOnce(opts Options) (string, Result, error) {
	return runOnce(opts, s.toolRunnerFromOptions)
}

func (s *Service) toolRunnerFromOptions(opts Options) (ToolRunner, error) {
	tool := strings.TrimSpace(opts.Tool)
	if tool == "" {
		tool = "codex"
	}
	tr, ok := s.runners[tool]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
	return tr, nil
}
