package runner

// ToolRunner executes a crawl tool (e.g. Codex CLI, Gemini CLI) and returns the raw
// JSON output as a string.
type ToolRunner interface {
	Name() string
	Run(url string, prompt string) (raw string, err error)
	CheckAuth() AuthCheck
}

// AuthCheck reports both file-based auth state and a network probe.
// Errors are surfaced as strings to keep the interface lightweight.
type AuthCheck struct {
	FilePath  string
	FileExists bool
	FileErr   string
	NetworkOK bool
	NetworkErr string
}
