package crawl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

func EventIDForURL(rawURL string) (string, error) {
	norm, err := normalizeURLForDedupe(rawURL)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(norm))
	return "crawl:shopee:" + hex.EncodeToString(sum[:]), nil
}

func normalizeURLForDedupe(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if strings.TrimSpace(u.Hostname()) == "" {
		return "", fmt.Errorf("invalid URL (missing host): %q", rawURL)
	}

	host := strings.ToLower(u.Hostname())
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}

	// Drop query + fragment to avoid tracking params and model selection noise.
	return "https://" + host + path, nil
}

