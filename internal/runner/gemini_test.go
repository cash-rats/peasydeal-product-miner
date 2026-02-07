package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestGeminiRunner_Run_RetriesOnTruncation(t *testing.T) {
	t.Parallel()

	// First run: wrapper JSON, but response is truncated mid-object (invalid JSON).
	// Second run: wrapper JSON with valid contract JSON.
	outputs := []string{
		`{"session_id":"s1","response":"{\"status\":\"ok\",\"url\":\"https://example.com\",\"captured_at\":\"2026-01-01T00:00:00Z\",\"title\":\"t\",\"description\":\"d\",\"currency\":\"TWD\",\"price\":\"1\",\"images\":[\"x\"],\"variations\":[{\"title\":\"v\",\"position\":0,\"image\":\"i\"}],\"truncated\":\"`,
		`{"session_id":"s2","response":"{\"status\":\"ok\",\"url\":\"https://example.com\",\"captured_at\":\"2026-01-01T00:00:00Z\",\"title\":\"t\",\"description\":\"d\",\"currency\":\"TWD\",\"price\":\"1\",\"images\":[],\"variations\":[]}","stats":{}}`,
	}
	exits := []int{0, 0}

	var calls [][]string
	callIdx := 0

	r := NewGeminiRunner(GeminiRunnerConfig{
		Cmd:    "gemini",
		Model:  "test-model",
		Logger: zap.NewNop().Sugar(),
	})

	r.execCommand = func(_ string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))

		cmd := exec.Command(os.Args[0], "-test.run=TestGeminiRunnerHelperProcess", "--")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("HELPER_STDOUT=%s", outputs[callIdx]),
			"HELPER_STDERR=",
			fmt.Sprintf("HELPER_EXIT=%d", exits[callIdx]),
		)
		callIdx++
		return cmd
	}

	got, err := r.Run("https://example.com", "original prompt")
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if _, perr := extractJSONObjectWithStatus(got); perr != nil {
		t.Fatalf("expected contract JSON, got error: %v output=%q", perr, got)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (retry on truncation), got %d", len(calls))
	}
	if calls[0][len(calls[0])-1] != "original prompt" {
		t.Fatalf("unexpected first prompt: %q", calls[0][len(calls[0])-1])
	}
	if !strings.Contains(calls[1][len(calls[1])-1], "Output limits") {
		t.Fatalf("expected retry prompt to include output limits, got: %q", calls[1][len(calls[1])-1])
	}
}

func TestGeminiRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	_, _ = os.Stdout.WriteString(os.Getenv("HELPER_STDOUT"))
	_, _ = os.Stderr.WriteString(os.Getenv("HELPER_STDERR"))

	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code)
}

