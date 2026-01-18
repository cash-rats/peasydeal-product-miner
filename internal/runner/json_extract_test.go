package runner

import "testing"

func TestExtractFirstJSONObject_MarkdownFence(t *testing.T) {
	out, err := extractFirstJSONObject("```json\n{\n  \"a\": 1\n}\n```")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != `{"a":1}` {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestExtractFirstJSONObject_LeadingTrailingText(t *testing.T) {
	out, err := extractFirstJSONObject("hello\n  {\"a\":1}\nbye")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != `{"a":1}` {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestExtractFirstJSONObject_MultipleObjects(t *testing.T) {
	out, err := extractFirstJSONObject("{\"a\":1}\n{\"b\":2}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != `{"a":1}` {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestExtractFirstJSONObject_BracesInsideStrings(t *testing.T) {
	out, err := extractFirstJSONObject("prefix {\"a\":\"{brace} and } in string\"} suffix {\"b\":2}")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out != `{"a":"{brace} and } in string"}` {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestExtractFirstJSONObject_NoJSON(t *testing.T) {
	_, err := extractFirstJSONObject("no json here")
	if err == nil {
		t.Fatalf("expected error")
	}
}
