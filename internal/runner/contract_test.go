package runner

import (
	"encoding/json"
	"testing"
)

func TestValidateCrawlOut_StatusOkRequiresFields(t *testing.T) {
	out := CrawlOut{
		URL:        "https://example.com",
		Status:     "ok",
		CapturedAt: "2026-01-18T00:00:00Z",
		Currency:   "TWD",
		Price:      json.Number("123.45"),
		Title:      "",
		// Description missing too
	}
	if err := validateCrawlOut(out); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateCrawlOut_StatusNeedsManualRequiresNotes(t *testing.T) {
	out := CrawlOut{
		URL:        "https://example.com",
		Status:     "needs_manual",
		CapturedAt: "2026-01-18T00:00:00Z",
	}
	if err := validateCrawlOut(out); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateCrawlOut_StatusErrorRequiresError(t *testing.T) {
	out := CrawlOut{
		URL:        "https://example.com",
		Status:     "error",
		CapturedAt: "2026-01-18T00:00:00Z",
	}
	if err := validateCrawlOut(out); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestValidateCrawlOut_PriceAllowsNumericString(t *testing.T) {
	out := CrawlOut{
		URL:         "https://example.com",
		Status:      "ok",
		CapturedAt:  "2026-01-18T00:00:00Z",
		Title:       "t",
		Description: "d",
		Currency:    "TWD",
		Price:       "123.45",
	}
	if err := validateCrawlOut(out); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateCrawlOut_ImagesOptional(t *testing.T) {
	out := CrawlOut{
		URL:         "https://example.com",
		Status:      "ok",
		CapturedAt:  "2026-01-18T00:00:00Z",
		Title:       "t",
		Description: "d",
		Currency:    "TWD",
		Price:       json.Number("123.45"),
		// Images intentionally omitted.
	}
	if err := validateCrawlOut(out); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateCrawlOut_PriceRejectsNonNumericString(t *testing.T) {
	out := CrawlOut{
		URL:         "https://example.com",
		Status:      "ok",
		CapturedAt:  "2026-01-18T00:00:00Z",
		Title:       "t",
		Description: "d",
		Currency:    "TWD",
		Price:       "12,345",
	}
	if err := validateCrawlOut(out); err == nil {
		t.Fatalf("expected validation error")
	}
}
