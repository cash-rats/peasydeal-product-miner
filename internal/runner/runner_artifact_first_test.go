package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type stubToolRunner struct {
	name     string
	raw      string
	runErr   error
	authErr  error
	runCalls int
}

func (s *stubToolRunner) Name() string { return s.name }

func (s *stubToolRunner) Run(_ string, _ string) (string, error) {
	s.runCalls++
	return s.raw, s.runErr
}

func (s *stubToolRunner) CheckAuth() error { return s.authErr }

func TestRunOnce_OrchestratorSkill_UsesFinalArtifactEvenWhenToolOutputInvalid(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	runID := "run-1"
	artifactDir := filepath.Join(outDir, "artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	final := `{"url":"https://shopee.tw/i.1.2","status":"ok","captured_at":"2026-01-01T00:00:00Z","title":"t","description":"d","currency":"TWD","price":"1","images":[],"variations":[]}`
	if err := os.WriteFile(filepath.Join(artifactDir, "final.json"), []byte(final), 0o644); err != nil {
		t.Fatalf("write final.json: %v", err)
	}

	tool := &stubToolRunner{
		name:   "gemini",
		raw:    "not-json",
		runErr: fmt.Errorf("gemini returned non-JSON output"),
	}

	r := &Runner{
		logger:    zap.NewNop().Sugar(),
		runners:   map[string]ToolRunner{"gemini": tool},
		validator: validator.New(),
	}

	_, res, err := r.RunOnce(Options{
		URL:       "https://shopee.tw/i.1.2",
		OutDir:    outDir,
		Tool:      "gemini",
		SkillName: shopeeOrchestratorPipelineSkill,
		RunID:     runID,
	})
	if err != nil {
		t.Fatalf("RunOnce error: %v", err)
	}
	if got, _ := res["status"].(string); got != "ok" {
		t.Fatalf("unexpected status: %#v", res["status"])
	}
	if got, _ := res["result_source"].(string); got != "artifact_final" {
		t.Fatalf("unexpected result_source: %#v", res["result_source"])
	}
}

func TestRunOnce_OrchestratorSkill_FinalArtifactStatusErrorReturnsError(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	runID := "run-2"
	artifactDir := filepath.Join(outDir, "artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	final := `{"url":"https://shopee.tw/i.1.2","status":"error","captured_at":"2026-01-01T00:00:00Z","error":"pipeline failed","images":[],"variations":[]}`
	if err := os.WriteFile(filepath.Join(artifactDir, "final.json"), []byte(final), 0o644); err != nil {
		t.Fatalf("write final.json: %v", err)
	}

	tool := &stubToolRunner{name: "gemini", raw: "ignored"}
	r := &Runner{
		logger:    zap.NewNop().Sugar(),
		runners:   map[string]ToolRunner{"gemini": tool},
		validator: validator.New(),
	}

	_, res, err := r.RunOnce(Options{
		URL:       "https://shopee.tw/i.1.2",
		OutDir:    outDir,
		Tool:      "gemini",
		SkillName: shopeeOrchestratorPipelineSkill,
		RunID:     runID,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "status error") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := res["status"].(string); got != "error" {
		t.Fatalf("unexpected result status: %#v", res["status"])
	}
}
