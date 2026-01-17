package chromedevtools

import (
	"context"
	"net"
	"testing"
)

func TestVersionURLResolved_InDocker_ResolvesToIPv4(t *testing.T) {
	origInDocker := inDockerFunc
	origLookup := lookupIPAddrs
	t.Cleanup(func() {
		inDockerFunc = origInDocker
		lookupIPAddrs = origLookup
	})

	inDockerFunc = func() bool { return true }
	lookupIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if host != "host.docker.internal" {
			t.Fatalf("unexpected lookup host: %q", host)
		}
		return []net.IPAddr{
			{IP: net.ParseIP("::1")},
			{IP: net.ParseIP("192.0.2.10")},
		}, nil
	}

	url, effectiveHost := VersionURLResolved(context.Background(), "", "")
	if effectiveHost != "192.0.2.10" {
		t.Fatalf("expected effectiveHost=192.0.2.10, got %q", effectiveHost)
	}
	if want := "http://192.0.2.10:9222/json/version"; url != want {
		t.Fatalf("expected url=%q, got %q", want, url)
	}
}

func TestVersionURLResolved_InDocker_IPLiteral_NoLookup(t *testing.T) {
	origInDocker := inDockerFunc
	origLookup := lookupIPAddrs
	t.Cleanup(func() {
		inDockerFunc = origInDocker
		lookupIPAddrs = origLookup
	})

	inDockerFunc = func() bool { return true }
	lookupIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		t.Fatalf("lookup should not be called for IP literal, got host=%q", host)
		return nil, nil
	}

	url, effectiveHost := VersionURLResolved(context.Background(), "10.0.0.1", "9222")
	if effectiveHost != "10.0.0.1" {
		t.Fatalf("expected effectiveHost=10.0.0.1, got %q", effectiveHost)
	}
	if want := "http://10.0.0.1:9222/json/version"; url != want {
		t.Fatalf("expected url=%q, got %q", want, url)
	}
}

func TestVersionURLResolved_NotInDocker_DoesNotResolve(t *testing.T) {
	origInDocker := inDockerFunc
	origLookup := lookupIPAddrs
	t.Cleanup(func() {
		inDockerFunc = origInDocker
		lookupIPAddrs = origLookup
	})

	inDockerFunc = func() bool { return false }
	lookupIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		t.Fatalf("lookup should not be called when not in docker, got host=%q", host)
		return nil, nil
	}

	url, effectiveHost := VersionURLResolved(context.Background(), "host.docker.internal", "9222")
	if effectiveHost != "host.docker.internal" {
		t.Fatalf("expected effectiveHost=host.docker.internal, got %q", effectiveHost)
	}
	if want := "http://host.docker.internal:9222/json/version"; url != want {
		t.Fatalf("expected url=%q, got %q", want, url)
	}
}
