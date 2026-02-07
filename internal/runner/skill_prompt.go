package runner

import (
	"fmt"
	"strings"

	"peasydeal-product-miner/internal/source"
)

const shopeeProductCrawlerSkill = "shopee-product-crawler"

func buildSkillPrompt(src source.Source, url string, skillName string) (string, error) {
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

	return fmt.Sprintf(`
Use the "%s" skill as the primary crawling guide.

Target URL: %s

Return EXACTLY ONE JSON object that matches the crawler output contract:
{
  "url": "string",
  "status": "ok | needs_manual | error",
  "captured_at": "ISO-8601 UTC timestamp",
  "notes": "string (required when status=needs_manual)",
  "error": "string (required when status=error)",
  "title": "string",
  "description": "string",
  "currency": "string (e.g. TWD)",
  "price": "number or numeric string",
  "images": ["string"] (optional; empty array allowed),
  "variations": [
    {
      "title": "string",
      "position": "int",
      "image": "string"
    }
  ]
}

Rules:
- Output JSON only.
- No markdown fences.
- No extra prose.
- If blocked by login/verification/CAPTCHA and product content is unavailable, return status="needs_manual" with notes.
`, skillName, url), nil
}

func defaultSkillName(src source.Source) string {
	switch src {
	case source.Shopee:
		return shopeeProductCrawlerSkill
	default:
		return ""
	}
}
