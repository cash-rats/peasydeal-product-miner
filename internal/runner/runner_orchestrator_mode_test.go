package runner

import (
	"testing"

	"peasydeal-product-miner/internal/source"
)

func TestIsOrchestratorSkillMode_ShopeeDefault(t *testing.T) {
	t.Parallel()

	ok := isOrchestratorSkillMode(Options{PromptMode: promptModeSkill}, source.Shopee)
	if !ok {
		t.Fatalf("expected shopee default skill-mode to be orchestrator")
	}
}

func TestIsOrchestratorSkillMode_TaobaoDefault(t *testing.T) {
	t.Parallel()

	ok := isOrchestratorSkillMode(Options{PromptMode: promptModeSkill}, source.Taobao)
	if !ok {
		t.Fatalf("expected taobao default skill-mode to be orchestrator")
	}
}

func TestIsOrchestratorSkillMode_RejectsUnknownSkill(t *testing.T) {
	t.Parallel()

	ok := isOrchestratorSkillMode(Options{PromptMode: promptModeSkill, SkillName: "shopee-page-snapshot"}, source.Shopee)
	if ok {
		t.Fatalf("expected non-orchestrator skill to be rejected")
	}
}
