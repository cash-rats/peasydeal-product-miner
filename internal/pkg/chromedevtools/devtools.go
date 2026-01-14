package chromedevtools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

	client := &http.Client{Timeout: timeout}
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
