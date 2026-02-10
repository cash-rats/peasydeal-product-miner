package runner

import (
	"encoding/json"
	"testing"
)

func TestValidateCrawlOut_StatusOkAllowsMissingCoreFields(t *testing.T) {
	out := CrawlOut{
		URL:        "https://example.com",
		Status:     "ok",
		CapturedAt: "2026-01-18T00:00:00Z",
		// Core fields intentionally omitted to allow degraded outputs.
	}
	if err := validateCrawlOut(out); err != nil {
		t.Fatalf("expected no error, got %v", err)
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

func TestValidateCrawlOut_VariationImagesArray(t *testing.T) {
	out := CrawlOut{
		URL:         "https://example.com",
		Status:      "ok",
		CapturedAt:  "2026-01-18T00:00:00Z",
		Title:       "t",
		Description: "d",
		Currency:    "TWD",
		Price:       "123.45",
		Variations: []Variation{
			{
				Title:    "v",
				Position: 0,
				Images:   []string{"https://img/v0-1", "https://img/v0-2"},
			},
		},
	}
	if err := validateCrawlOut(out); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
