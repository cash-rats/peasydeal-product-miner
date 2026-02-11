package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"go.uber.org/zap"
)

func TestGeminiRunner_Run_NoRetryWhenInvalidJSON(t *testing.T) {
	t.Parallel()

	outputs := []string{
		`{"session_id":"s1","response":"{\"status\":\"ok\",\"url\":\"https://example.com\"","stats":{}}`,
	}
	exits := []int{0}

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

	_, err := r.Run("https://example.com", "original prompt")
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(calls))
	}
}

func TestGeminiRunner_Run_ValidJSON(t *testing.T) {
	t.Parallel()

	outputs := []string{
		`{"session_id":"s2","response":"{\"status\":\"ok\",\"url\":\"https://example.com\",\"captured_at\":\"2026-01-01T00:00:00Z\",\"title\":\"t\",\"description\":\"d\",\"currency\":\"TWD\",\"price\":\"1\",\"images\":[],\"variations\":[]}","stats":{}}`,
	}
	exits := []int{0}

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
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(calls))
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
