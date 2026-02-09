package runner

import (
	"fmt"
	"path/filepath"
	"strings"

	"peasydeal-product-miner/internal/source"
)

const shopeeProductCrawlerSkill = "shopee-product-crawler"

func buildSkillPrompt(src source.Source, url string, skillName string, tool string, runID string, outDir string) (string, error) {
	skillName = strings.TrimSpace(skillName)
	if skillName == "" {
		skillName = defaultSkillName(src)
	}
	if skillName == "" {
		return "", fmt.Errorf("no default skill for source %q", src)
	}

	if src != source.Shopee {
		return "", fmt.Errorf("prompt_mode=skill is currently supported for shopee only (source=%q)", src)
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

	// Gemini CLI skill triggering is somewhat sensitive to phrasing; use the most direct wording we observed
	// to reliably activate the intended skill in headless runs.
	if strings.EqualFold(skillName, "shopee-page-snapshot") {
		return fmt.Sprintf(`Please use shopee-page-snapshot on %s. Follow the skill instructions exactly and return JSON only.%s`, url, tail.String()), nil
	}

	return fmt.Sprintf(`Use the "%s" skill as the primary crawling guide. Target URL: %s%s`, skillName, url, tail.String()), nil
}

func defaultSkillName(src source.Source) string {
	switch src {
	case source.Shopee:
		return shopeeProductCrawlerSkill
	default:
		return ""
	}
}
