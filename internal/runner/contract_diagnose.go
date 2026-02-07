package runner

import (
	"encoding/json"
	"strings"
)

// diagnoseContractIssue returns a stable, user-facing parse diagnosis for logging.
// It avoids treating nested JSON fragments as the top-level output object.
func diagnoseContractIssue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "empty response"
	}

	// Handle markdown fence wrappers.
	if strings.HasPrefix(raw, "```") {
		if fenced := extractFirstMarkdownFence(raw); fenced != "" {
			raw = strings.TrimSpace(fenced)
		}
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()

	var top any
	if err := dec.Decode(&top); err != nil {
		return "invalid or truncated JSON"
	}

	// Ensure no trailing non-whitespace tokens.
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return "multiple top-level JSON values"
	}

	obj, ok := top.(map[string]any)
	if !ok {
		return "top-level JSON is not an object"
	}

	status, ok := obj["status"].(string)
	if !ok || strings.TrimSpace(status) == "" {
		return "missing status"
	}

	return "unknown contract issue"
}
