package runner

import (
	"os"
	"strings"
	"testing"

	"peasydeal-product-miner/internal/source"
)

func TestNormalizeOptions_DefaultsToLegacyMode(t *testing.T) {
	t.Setenv("CRAWL_PROMPT_MODE", "")
	opts := normalizeOptions(Options{
		URL:    "https://shopee.tw/product/1/2",
		OutDir: "out",
	})
	if opts.PromptMode != promptModeLegacy {
		t.Fatalf("expected prompt mode %q, got %q", promptModeLegacy, opts.PromptMode)
	}
}

func TestNormalizeOptions_UsesEnvPromptMode(t *testing.T) {
	t.Setenv("CRAWL_PROMPT_MODE", "skill")
	opts := normalizeOptions(Options{
		URL:    "https://shopee.tw/product/1/2",
		OutDir: "out",
	})
	if opts.PromptMode != promptModeSkill {
		t.Fatalf("expected prompt mode %q, got %q", promptModeSkill, opts.PromptMode)
	}
}

func TestBuildSkillPrompt_Shopee(t *testing.T) {
	got, err := buildSkillPrompt(source.Shopee, "https://shopee.tw/product/1/2", "", "codex", "", "out")
	if err != nil {
		t.Fatalf("buildSkillPrompt error: %v", err)
	}
	if !strings.Contains(got, shopeeOrchestratorPipelineSkill) {
		t.Fatalf("expected skill name in prompt: %s", got)
	}
	if !strings.Contains(got, "https://shopee.tw/product/1/2") {
		t.Fatalf("expected URL in prompt: %s", got)
	}
	if !strings.Contains(got, "Use the \"shopee-orchestrator-pipeline\" skill as the primary crawling guide") {
		t.Fatalf("expected skill invocation prompt: %s", got)
	}
}

func TestBuildSkillPrompt_RejectsUnsupportedShopeeSkill(t *testing.T) {
	_, err := buildSkillPrompt(
		source.Shopee,
		"https://shopee.tw/product/1/2",
		"shopee-page-snapshot",
		"gemini",
		"",
		"out",
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildSkillPrompt_Taobao(t *testing.T) {
	got, err := buildSkillPrompt(source.Taobao, "https://item.taobao.com/item.htm?id=1", "", "codex", "", "out")
	if err != nil {
		t.Fatalf("buildSkillPrompt error: %v", err)
	}
	if !strings.Contains(got, taobaoOrchestratorPipelineSkill) {
		t.Fatalf("expected skill name in prompt: %s", got)
	}
	if !strings.Contains(got, "https://item.taobao.com/item.htm?id=1") {
		t.Fatalf("expected URL in prompt: %s", got)
	}
}

func TestBuildSkillPrompt_RejectsUnsupportedTaobaoSkill(t *testing.T) {
	_, err := buildSkillPrompt(
		source.Taobao,
		"https://item.taobao.com/item.htm?id=1",
		"taobao-page-snapshot",
		"codex",
		"",
		"out",
	)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildSkillPrompt_UnsupportedSource(t *testing.T) {
	_, err := buildSkillPrompt(source.Source("unknown"), "https://example.com/item/1", "", "codex", "", "out")
	if err == nil {
		t.Fatalf("expected error for unsupported source")
	}
}

func TestBuildSkillPrompt_WithRunID(t *testing.T) {
	got, err := buildSkillPrompt(
		source.Shopee,
		"https://shopee.tw/product/1/2",
		"shopee-orchestrator-pipeline",
		"gemini",
		"run-123",
		"out",
	)
	if err != nil {
		t.Fatalf("buildSkillPrompt error: %v", err)
	}
	if !strings.Contains(got, "Run ID: run-123") {
		t.Fatalf("expected run id in prompt, got: %s", got)
	}
	if !strings.Contains(got, "Artifact dir: out/artifacts/run-123") {
		t.Fatalf("expected artifact dir in prompt, got: %s", got)
	}
}

func TestBuildPrompt_SkillModeRejectsPromptFile(t *testing.T) {
	_, err := buildPrompt(Options{
		URL:        "https://shopee.tw/product/1/2",
		OutDir:     "out",
		PromptMode: promptModeSkill,
		PromptFile: "config/prompt.shopee.product.txt",
	}, source.Shopee)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestResolveRunnerWorkDirFromEnv(t *testing.T) {
	t.Setenv("RUNNER_WORKDIR", "/tmp/example")
	got := resolveRunnerWorkDir("")
	if got != "/tmp/example" {
		t.Fatalf("expected /tmp/example, got %q", got)
	}
}

func TestResolveRunnerWorkDirPrefersExplicit(t *testing.T) {
	t.Setenv("RUNNER_WORKDIR", "/tmp/env")
	got := resolveRunnerWorkDir("/tmp/explicit")
	if got != "/tmp/explicit" {
		t.Fatalf("expected /tmp/explicit, got %q", got)
	}
}

func TestFindProjectRootFrom(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := findProjectRootFrom(wd)
	if root == "" {
		t.Fatalf("expected project root, got empty")
	}
}
