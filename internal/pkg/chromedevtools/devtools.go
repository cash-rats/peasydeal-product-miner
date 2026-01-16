package chromedevtools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	neturl "net/url"
)

const DefaultHost = "127.0.0.1"
const DefaultPort = "9222"

func VersionURL(host, port string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = DefaultHost
	}
	port = strings.TrimSpace(port)
	if port == "" {
		port = DefaultPort
	}
	return fmt.Sprintf("http://%s:%s/json/version", host, port)
}

func VersionURLResolved(ctx context.Context, host, port string) (string, string) {
	host = strings.TrimSpace(host)
	if host == "" {
		if InDocker() {
			host = "host.docker.internal"
		} else {
			host = DefaultHost
		}
	}
	port = strings.TrimSpace(port)
	if port == "" {
		port = DefaultPort
	}

	effectiveHost := host
	if InDocker() {
		if ip, ok := resolveHostToIPv4(ctx, host); ok {
			effectiveHost = ip
		}
	}

	return fmt.Sprintf("http://%s:%s/json/version", effectiveHost, port), effectiveHost
}

// InDocker returns true if the current process appears to be running inside a Docker container.
func InDocker() bool {
	return inDockerFunc()
}

var inDockerFunc = func() bool {
	// Minimal and low-risk heuristic; avoid clever detection.
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

var lookupIPAddrs = net.DefaultResolver.LookupIPAddr

func resolveHostToIPv4(ctx context.Context, host string) (string, bool) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", false
	}

	// If already an IP literal, keep it.
	if ip := net.ParseIP(host); ip != nil {
		return host, true
	}

	// If host is a URL, extract the hostname portion.
	if u, err := neturl.Parse(host); err == nil && u.Host != "" {
		host = u.Hostname()
		if host == "" {
			return "", false
		}
	}

	addrs, err := lookupIPAddrs(ctx, host)
	if err != nil || len(addrs) == 0 {
		return "", false
	}

	// Prefer IPv4 if present.
	for _, addr := range addrs {
		if ip := addr.IP.To4(); ip != nil {
			return ip.String(), true
		}
	}

	// Fall back to first result (likely IPv6) if no v4 exists.
	return addrs[0].IP.String(), true
}

func CheckReachable(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("missing url")
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := newHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("unexpected status %s from %s", resp.Status, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*32))
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, fmt.Errorf("empty response from %s", url)
	}

	return body, nil
}

var newHTTPClient = func(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
