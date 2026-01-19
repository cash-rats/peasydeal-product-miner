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

func TestCodexRunner_Run_ReturnsExtractedJSON_NoRepair(t *testing.T) {
	t.Parallel()

	var calls [][]string
	outputs := []string{`{"ok":true}`}
	exits := []int{0}

	r := NewCodexRunner(CodexRunnerConfig{
		Cmd:              "codex",
		Model:            "test-model",
		SkipGitRepoCheck: true,
		Logger:           zap.NewNop().Sugar(),
	})

	callIdx := 0
	r.execCommand = func(_ string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))

		cmd := exec.Command(os.Args[0], "-test.run=TestCodexRunnerHelperProcess", "--")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("HELPER_STDOUT=%s", outputs[callIdx]),
			"HELPER_STDERR=",
			fmt.Sprintf("HELPER_EXIT=%d", exits[callIdx]),
		)
		callIdx++
		return cmd
	}

	got, err := r.Run("https://example.com", `{"prompt":"x"}`)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got != `{"ok":true}` {
		t.Fatalf("unexpected output: %s", got)
	}

	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	args := calls[0]
	if !containsAll(args, []string{"exec", "--skip-git-repo-check", "--model", "test-model", `{"prompt":"x"}`}) {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestCodexRunner_Run_RepairsOnce(t *testing.T) {
	t.Parallel()

	var calls [][]string
	outputs := []string{"not json", `{"repaired":true}`}
	exits := []int{0, 0}

	url := "https://example.com/p/1"
	prompt := "original prompt"

	r := NewCodexRunner(CodexRunnerConfig{
		Cmd:              "codex",
		Model:            "test-model",
		SkipGitRepoCheck: true,
		Logger:           zap.NewNop().Sugar(),
	})

	callIdx := 0
	r.execCommand = func(_ string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))

		cmd := exec.Command(os.Args[0], "-test.run=TestCodexRunnerHelperProcess", "--")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("HELPER_STDOUT=%s", outputs[callIdx]),
			"HELPER_STDERR=",
			fmt.Sprintf("HELPER_EXIT=%d", exits[callIdx]),
		)
		callIdx++
		return cmd
	}

	got, err := r.Run(url, prompt)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got != `{"repaired":true}` {
		t.Fatalf("unexpected output: %s", got)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0][len(calls[0])-1] != prompt {
		t.Fatalf("unexpected first prompt: %q", calls[0][len(calls[0])-1])
	}

	repairPrompt := calls[1][len(calls[1])-1]
	if !strings.Contains(repairPrompt, url) || !strings.Contains(repairPrompt, "not json") {
		t.Fatalf("unexpected repair prompt: %q", repairPrompt)
	}
}

func TestCodexRunner_Run_RepairFails(t *testing.T) {
	t.Parallel()

	outputs := []string{"not json", "still not json"}
	exits := []int{0, 0}

	r := NewCodexRunner(CodexRunnerConfig{
		Cmd:              "codex",
		Model:            "test-model",
		SkipGitRepoCheck: true,
		Logger:           zap.NewNop().Sugar(),
	})

	callIdx := 0
	r.execCommand = func(_ string, args ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestCodexRunnerHelperProcess", "--")
		cmd.Env = append(os.Environ(),
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("HELPER_STDOUT=%s", outputs[callIdx]),
			"HELPER_STDERR=",
			fmt.Sprintf("HELPER_EXIT=%d", exits[callIdx]),
		)
		callIdx++
		return cmd
	}

	_, err := r.Run("https://example.com", "prompt")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "codex returned non-JSON output") {
		t.Fatalf("unexpected error: %v", err)
	}
	if callIdx != 2 {
		t.Fatalf("expected 2 calls, got %d", callIdx)
	}
}

func TestCodexRunnerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	_, _ = os.Stdout.WriteString(os.Getenv("HELPER_STDOUT"))
	_, _ = os.Stderr.WriteString(os.Getenv("HELPER_STDERR"))

	code, _ := strconv.Atoi(os.Getenv("HELPER_EXIT"))
	os.Exit(code)
}

func containsAll(args []string, want []string) bool {
	for _, w := range want {
		found := false
		for _, a := range args {
			if a == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

