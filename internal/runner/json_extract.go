package runner

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractFirstJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty response")
	}

	// Handle markdown fences like ```json ... ```
	if strings.HasPrefix(raw, "```") {
		if fenced := extractFirstMarkdownFence(raw); fenced != "" {
			raw = strings.TrimSpace(fenced)
		}
	}

	// Scan for a syntactically valid JSON object starting at each '{'.
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}

		dec := json.NewDecoder(strings.NewReader(raw[i:]))
		dec.UseNumber()

		var v any
		if err := dec.Decode(&v); err != nil {
			continue
		}

		// Ensure top-level is an object
		if _, ok := v.(map[string]any); !ok {
			continue
		}

		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	return "", fmt.Errorf("no valid JSON object found")
}

func extractFirstMarkdownFence(s string) string {
	const fence = "```"
	start := strings.Index(s, fence)
	if start < 0 {
		return ""
	}
	s = s[start+len(fence):]

	// Optional language tag (e.g. "json") until first newline.
	if nl := strings.IndexByte(s, '\n'); nl >= 0 {
		s = s[nl+1:]
	} else {
		return ""
	}

	end := strings.Index(s, fence)
	if end < 0 {
		return ""
	}
	return s[:end]
}
