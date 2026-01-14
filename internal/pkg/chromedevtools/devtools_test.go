package chromedevtools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckReachable_OK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Browser":"Chrome"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := CheckReachable(context.Background(), srv.URL, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestCheckReachable_EmptyBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("   \n\t  "))
	}))
	t.Cleanup(srv.Close)

	_, err := CheckReachable(context.Background(), srv.URL, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestCheckReachable_Non2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, err := CheckReachable(context.Background(), srv.URL, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestCheckReachable_Timeout(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	_, err := CheckReachable(context.Background(), srv.URL, 50*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

