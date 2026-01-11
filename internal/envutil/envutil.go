package envutil

import "strings"

// String returns the trimmed value of the environment variable, or def if empty.
func String(getenv func(string) string, key string, def string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return def
}

// Bool parses common boolean environment variable values, returning def on empty/unknown.
func Bool(getenv func(string) string, key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}
