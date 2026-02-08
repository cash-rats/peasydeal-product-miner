package runner

import (
	"fmt"
	"strings"

	"peasydeal-product-miner/internal/source"
)

const shopeeProductCrawlerSkill = "shopee-product-crawler"

func buildSkillPrompt(src source.Source, url string, skillName string, tool string) (string, error) {
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

	// Gemini CLI skill triggering is somewhat sensitive to phrasing; use the most direct wording we observed
	// to reliably activate the intended skill in headless runs.
	if strings.EqualFold(skillName, "shopee-page-snapshot") {
		return fmt.Sprintf(`Please use shopee-page-snapshot on %s. Follow the skill instructions exactly and return JSON only.`, url), nil
	}

	return fmt.Sprintf(`Use the "%s" skill as the primary crawling guide. Target URL: %s`, skillName, url), nil
}

func defaultSkillName(src source.Source) string {
	switch src {
	case source.Shopee:
		return shopeeProductCrawlerSkill
	default:
		return ""
	}
}
