# DevTools Reachability in Docker: Resolution + Fallback Plan

## Problem
The crawler currently checks Chrome DevTools reachability by requesting the configured URL directly (e.g. `http://host.docker.internal:9222/json/version`). In some environments this can fail (or return non-2xx) even though the host’s DevTools is reachable, especially when hostname resolution or IPv6/IPv4 selection is inconsistent.

## Goals
- Keep the current “try the configured URL first” behavior.
- Add a robust fallback that resolves the hostname to IP(s) and retries against the resolved IP(s).
- Prefer IPv4 when multiple addresses exist.
- Improve observability: make errors/logs show what was tried.
- Keep changes localized to `internal/pkg/chromedevtools` and tests.

## Implementation Steps
1. **Review current flow**
   - Identify where `chromedevtools.VersionURL()` and `chromedevtools.CheckReachable()` are used (e.g. `internal/app/inngest/crawl/crawl.go`).
   - Confirm config sources for `CHROME_DEBUG_HOST` / `CHROME_DEBUG_PORT` and their defaults.

2. **Add Docker environment detection**
   - Add `chromedevtools.InDocker() bool` using a low-risk heuristic:
     - Prefer checking for `/.dockerenv`.
     - Optionally add a cgroup fallback (only if needed).
   - Use this only to choose safer defaults when host is empty; do not override an explicitly configured host.

3. **Add hostname-to-IP resolution helper**
   - Add `resolveHost(ctx, host) ([]net.IP, error)`:
     - If `host` is already an IP, return it.
     - Otherwise use `net.DefaultResolver.LookupIPAddr`.
     - Deduplicate results and sort with **IPv4 first**, then IPv6.

4. **Add targeted unit tests**
   - Add a test where:
     - `http://localhost:<port>/json/version` returns `500`
     - `http://127.0.0.1:<port>/json/version` returns `200`
     - This verifies IPv4 preference and validates resolution behavior in a controlled setup.
   - Keep existing tests and add only minimal new ones needed to cover the new behavior.

5. **Logging/doc adjustments**
   - Update the crawl step log to include the final effective DevTools endpoint (original host or resolved IP).
   - Add a short note to `README.md` describing the fallback behavior in Docker and how to override `CHROME_DEBUG_HOST`/`PORT` if needed.

## Acceptance Criteria
- The crawler uses safer defaults in Docker when host is unset (no surprises for local runs).
- Hostname resolution prefers IPv4 and behaves deterministically.
- `CheckReachable()` remains backward compatible for non-Docker/local usage.
- Unit tests cover the hostname resolution behavior.
- Logs make it obvious which endpoint was checked and what fallback occurred.
