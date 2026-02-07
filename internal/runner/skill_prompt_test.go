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
	got, err := buildSkillPrompt(source.Shopee, "https://shopee.tw/product/1/2", "", "codex")
	if err != nil {
		t.Fatalf("buildSkillPrompt error: %v", err)
	}
	if !strings.Contains(got, shopeeProductCrawlerSkill) {
		t.Fatalf("expected skill name in prompt: %s", got)
	}
	if !strings.Contains(got, "https://shopee.tw/product/1/2") {
		t.Fatalf("expected URL in prompt: %s", got)
	}
	if !strings.Contains(got, "Use the \"shopee-product-crawler\" skill as the primary crawling guide") {
		t.Fatalf("expected skill invocation prompt: %s", got)
	}
}

func TestBuildSkillPrompt_UnsupportedSource(t *testing.T) {
	_, err := buildSkillPrompt(source.Taobao, "https://taobao.com/item/1", "", "codex")
	if err == nil {
		t.Fatalf("expected error for unsupported source")
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
