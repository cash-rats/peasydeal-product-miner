package chromedevtools

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestCheckReachable_OK(t *testing.T) {
	orig := newHTTPClient
	t.Cleanup(func() { newHTTPClient = orig })

	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"Browser":"Chrome"}`)),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}

	_, err := CheckReachable(context.Background(), "http://example.test/json/version", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestCheckReachable_EmptyBody(t *testing.T) {
	orig := newHTTPClient
	t.Cleanup(func() { newHTTPClient = orig })

	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("   \n\t  ")),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}

	_, err := CheckReachable(context.Background(), "http://example.test/json/version", 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestCheckReachable_Non2xx(t *testing.T) {
	orig := newHTTPClient
	t.Cleanup(func() { newHTTPClient = orig })

	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Status:     "500 Internal Server Error",
					Body:       io.NopCloser(bytes.NewBufferString("nope")),
					Header:     make(http.Header),
				}, nil
			}),
		}
	}

	_, err := CheckReachable(context.Background(), "http://example.test/json/version", 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestCheckReachable_Timeout(t *testing.T) {
	orig := newHTTPClient
	t.Cleanup(func() { newHTTPClient = orig })

	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{
			Timeout: timeout,
			Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				select {
				case <-r.Context().Done():
					return nil, r.Context().Err()
				case <-time.After(200 * time.Millisecond):
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
						Header:     make(http.Header),
					}, nil
				}
			}),
		}
	}

	_, err := CheckReachable(context.Background(), "http://example.test/json/version", 50*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
