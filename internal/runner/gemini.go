package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type GeminiRunnerConfig struct {
	Cmd   string
	Model string
}

type GeminiRunner struct {
	cmd   string
	model string

	execCommand func(name string, args ...string) *exec.Cmd
}

func NewGeminiRunner(cfg GeminiRunnerConfig) *GeminiRunner {
	return &GeminiRunner{
		cmd:         cfg.Cmd,
		model:       cfg.Model,
		execCommand: exec.Command,
	}
}

func (r *GeminiRunner) Name() string { return "gemini" }

func (r *GeminiRunner) Run(url string, prompt string) (string, error) {
	// gemini [query..]
	// We use -o json to ensure we get parsable output.
	args := []string{"-o", "json"}
	if strings.TrimSpace(r.model) != "" {
		args = append(args, "--model", r.model)
	}
	// Allow the MCP server used by our DevTools-based prompts. Without this, Gemini CLI may deny
	// MCP tool calls in non-interactive mode due to policy.
	allowedMCP := strings.TrimSpace(os.Getenv("RUNNER_GEMINI_ALLOWED_MCP_SERVER_NAMES"))
	if allowedMCP == "" {
		allowedMCP = "chrome-devtools"
	}
	args = append(args, "--allowed-mcp-server-names", allowedMCP)
	args = append(args, prompt)

	start := time.Now()
	log.Printf("⏱️ crawl started tool=gemini url=%s", url)

	cmd := r.execCommand(r.cmd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if os.Getenv("RUNNER_DEBUG_GEMINI") == "1" {
			logGeminiDebugOutput(stdout.String(), stderr.String())
		}
		log.Printf("⏱️ crawl failed tool=gemini url=%s duration=%s err=%s", url, time.Since(start).Round(time.Millisecond), msg)
		return "", fmt.Errorf("gemini failed: %s", msg)
	}

	log.Printf("⏱️ crawl finished tool=gemini url=%s duration=%s", url, time.Since(start).Round(time.Millisecond))
	if os.Getenv("RUNNER_DEBUG_GEMINI") == "1" {
		logGeminiDebugOutput(stdout.String(), stderr.String())
	}

	raw := strings.TrimSpace(stdout.String())
	if unwrapped, ok := unwrapGeminiJSON(raw); ok {
		return unwrapped, nil
	}
	return raw, nil
}

func logGeminiDebugOutput(stdout string, stderr string) {
	const maxBytes = 4000
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if len(stdout) > maxBytes {
		stdout = stdout[:maxBytes] + "...(truncated)"
	}
	if len(stderr) > maxBytes {
		stderr = stderr[:maxBytes] + "...(truncated)"
	}
	log.Printf("🔎 gemini debug stdout=%q", stdout)
	if stderr != "" {
		log.Printf("🔎 gemini debug stderr=%q", stderr)
	}
}

// unwrapGeminiJSON extracts the tool's "response" field when Gemini is invoked with `-o json`.
// That output format is a wrapper object like:
//
//	{ "session_id": "...", "response": "<model text>", "stats": {...} }
//
// We want stdout to be the model text only (which should be the JSON contract).
func unwrapGeminiJSON(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return "", false
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", false
	}

	resp, ok := obj["response"]
	if !ok {
		return "", false
	}

	switch v := resp.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return "", false
		}
		return s, true
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			return "", false
		}
		return s, true
	default:
		return "", false
	}
}
