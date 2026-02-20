package runner

import (
	"testing"

	"peasydeal-product-miner/internal/source"
)

func TestIsOrchestratorSkillMode_ShopeeDefault(t *testing.T) {
	t.Parallel()

	ok := isOrchestratorSkillMode(Options{}, source.Shopee)
	if !ok {
		t.Fatalf("expected shopee default to be orchestrator")
	}
}

func TestIsOrchestratorSkillMode_TaobaoDefault(t *testing.T) {
	t.Parallel()

	ok := isOrchestratorSkillMode(Options{}, source.Taobao)
	if !ok {
		t.Fatalf("expected taobao default to be orchestrator")
	}
}

func TestIsOrchestratorSkillMode_RejectsUnknownSkill(t *testing.T) {
	t.Parallel()

	ok := isOrchestratorSkillMode(Options{SkillName: "shopee-page-snapshot"}, source.Shopee)
	if ok {
		t.Fatalf("expected non-orchestrator skill to be rejected")
	}
}
