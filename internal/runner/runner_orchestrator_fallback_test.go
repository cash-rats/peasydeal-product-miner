package runner

import (
	"os"
	"path/filepath"
	"testing"

	"peasydeal-product-miner/internal/source"
)

func TestLoadOrchestratorFinalResult_OK(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	runID := "run-1"
	artifactDir := filepath.Join(outDir, "artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	final := `{"url":"https://example.com","status":"ok","captured_at":"2026-01-01T00:00:00Z","title":"t","description":"d","currency":"TWD","price":"1","images":[],"variations":[]}`
	if err := os.WriteFile(filepath.Join(artifactDir, "final.json"), []byte(final), 0o644); err != nil {
		t.Fatalf("write final.json: %v", err)
	}

	res, err := loadOrchestratorFinalResult(Options{
		OutDir:     outDir,
		PromptMode: promptModeSkill,
		SkillName:  shopeeOrchestratorPipelineSkill,
		RunID:      runID,
	}, source.Shopee)
	if err != nil {
		t.Fatalf("loadOrchestratorFinalResult error: %v", err)
	}
	if res["status"] != "ok" {
		t.Fatalf("unexpected status: %#v", res["status"])
	}
	if res["result_source"] != "artifact_final" {
		t.Fatalf("unexpected result_source: %#v", res["result_source"])
	}
}

func TestLoadOrchestratorFinalResult_RejectsNonSkillMode(t *testing.T) {
	t.Parallel()

	_, err := loadOrchestratorFinalResult(Options{
		OutDir:     t.TempDir(),
		PromptMode: promptModeLegacy,
		SkillName:  shopeeOrchestratorPipelineSkill,
		RunID:      "run-1",
	}, source.Shopee)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadOrchestratorFinalResult_DefaultSkillNameForShopee(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	runID := "run-2"
	artifactDir := filepath.Join(outDir, "artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	final := `{"url":"https://example.com","status":"ok","captured_at":"2026-01-01T00:00:00Z","title":"t","description":"d","currency":"TWD","price":"1","images":[],"variations":[]}`
	if err := os.WriteFile(filepath.Join(artifactDir, "final.json"), []byte(final), 0o644); err != nil {
		t.Fatalf("write final.json: %v", err)
	}

	res, err := loadOrchestratorFinalResult(Options{
		OutDir:     outDir,
		PromptMode: promptModeSkill,
		RunID:      runID,
	}, source.Shopee)
	if err != nil {
		t.Fatalf("loadOrchestratorFinalResult error: %v", err)
	}
	if res["status"] != "ok" {
		t.Fatalf("unexpected status: %#v", res["status"])
	}
}

func TestLoadOrchestratorFinalResult_StatusErrorFails(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()
	runID := "run-3"
	artifactDir := filepath.Join(outDir, "artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	final := `{"url":"https://example.com","status":"error","captured_at":"2026-01-01T00:00:00Z","error":"stage failed","images":[],"variations":[]}`
	if err := os.WriteFile(filepath.Join(artifactDir, "final.json"), []byte(final), 0o644); err != nil {
		t.Fatalf("write final.json: %v", err)
	}

	_, err := loadOrchestratorFinalResult(Options{
		OutDir:     outDir,
		PromptMode: promptModeSkill,
		SkillName:  shopeeOrchestratorPipelineSkill,
		RunID:      runID,
	}, source.Shopee)
	if err == nil {
		t.Fatalf("expected error")
	}
}
