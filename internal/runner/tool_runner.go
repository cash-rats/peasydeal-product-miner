package runner

// ToolRunner executes a crawl tool (e.g. Codex CLI, Gemini CLI) and returns the raw
// JSON output as a string.
type ToolRunner interface {
	Name() string
	Run(url string, prompt string) (raw string, err error)
}
