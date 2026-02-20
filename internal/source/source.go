package source

import (
	"fmt"
	"net/url"
	"strings"
)

type Source string

const (
	Shopee Source = "shopee"
	Taobao Source = "taobao"
)

func Detect(rawURL string) (Source, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid URL (missing scheme/host): %q", rawURL)
	}

	host := strings.ToLower(u.Hostname())
	switch {
	case host == "shopee.tw" || strings.HasSuffix(host, ".shopee.tw"):
		return Shopee, nil
	case host == "taobao.com" || strings.HasSuffix(host, ".taobao.com"):
		return Taobao, nil
	case host == "tmall.com" || strings.HasSuffix(host, ".tmall.com"):
		return Taobao, nil
	default:
		return "", fmt.Errorf("unsupported URL host %q (only Shopee/Taobao/Tmall are supported)", host)
	}
}
