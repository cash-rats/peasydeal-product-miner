package runner

import "strings"

const (
	promptModeLegacy = "legacy"
	promptModeSkill  = "skill"
)

func normalizePromptMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case promptModeLegacy, promptModeSkill:
		return mode
	default:
		return mode
	}
}
