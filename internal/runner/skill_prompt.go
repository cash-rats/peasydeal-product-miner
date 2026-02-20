package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"peasydeal-product-miner/internal/source"
)

const shopeeOrchestratorPipelineSkill = "shopee-orchestrator-pipeline"
const taobaoOrchestratorPipelineSkill = "taobao-orchestrator-pipeline"

func buildSkillPrompt(src source.Source, url string, skillName string, tool string, runID string, outDir string) (string, error) {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		skillName = defaultSkillName(src)
	}
	if skillName == "" {
		return "", fmt.Errorf("no default skill for source %q", src)
	}

	switch src {
	case source.Shopee:
		if skillName != shopeeOrchestratorPipelineSkill {
			return "", fmt.Errorf("unsupported shopee skill %q (only %q is supported)", skillName, shopeeOrchestratorPipelineSkill)
		}
	case source.Taobao:
		if skillName != taobaoOrchestratorPipelineSkill {
			return "", fmt.Errorf("unsupported taobao skill %q (only %q is supported)", skillName, taobaoOrchestratorPipelineSkill)
		}
	default:
		return "", fmt.Errorf("prompt_mode=skill is unsupported for source=%q", src)
	}

	var tail strings.Builder
	runID = strings.TrimSpace(runID)
	if runID != "" {
		artifactDir := filepath.Join(strings.TrimSpace(outDir), "artifacts", runID)
		if strings.TrimSpace(outDir) == "" {
			artifactDir = filepath.Join("out", "artifacts", runID)
		}
		tail.WriteString("\n")
		tail.WriteString(fmt.Sprintf("Run ID: %s\n", runID))
		tail.WriteString(fmt.Sprintf("Artifact dir: %s\n", artifactDir))
		tail.WriteString("Use the provided Run ID exactly. Do not generate a new run_id.\n")
	}

	return fmt.Sprintf(`Use the "%s" skill as the primary crawling guide. Target URL: %s%s`, skillName, url, tail.String()), nil
}

func defaultSkillName(src source.Source) string {
	switch src {
	case source.Shopee:
		return shopeeOrchestratorPipelineSkill
	case source.Taobao:
		return taobaoOrchestratorPipelineSkill
	default:
		return ""
	}
}
