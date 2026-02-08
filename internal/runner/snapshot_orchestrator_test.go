package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCrawlResultFromSnapshot_OK(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// page_state.json with extracted core fields.
	pageState := map[string]any{
		"url": "https://example.com/p/1",
		"extracted": map[string]any{
			"title":       "t",
			"description": "d",
			"currency":    "TWD",
			"price":       "123",
		},
	}
	writeJSON(t, filepath.Join(dir, "page_state.json"), pageState)

	writeJSON(t, filepath.Join(dir, "overlay_images.json"), map[string]any{
		"images": []string{"https://img/1", "https://img/2"},
	})

	writeJSON(t, filepath.Join(dir, "variations.json"), map[string]any{
		"variations": []map[string]any{
			{"position": 0, "text": "v0"},
			{"position": 1, "text": "v1"},
		},
	})

	writeJSON(t, filepath.Join(dir, "variation_image_map.json"), map[string]any{
		"map": []map[string]any{
			{"text": "v0", "imageUrl": "https://img/v0"},
		},
	})

	ptr := SnapshotPointer{
		URL:         "https://example.com/p/1",
		Status:      "ok",
		CapturedAt:  "2026-01-01T00:00:00Z",
		RunID:       "r1",
		ArtifactDir: dir,
		SnapshotFiles: SnapshotFiles{
			PageState:         "page_state.json",
			OverlayImages:     "overlay_images.json",
			Variations:        "variations.json",
			VariationImageMap: "variation_image_map.json",
		},
	}

	res, err := buildCrawlResultFromSnapshot(ptr, "")
	if err != nil {
		t.Fatalf("buildCrawlResultFromSnapshot error: %v", err)
	}

	if res["status"] != "ok" {
		t.Fatalf("unexpected status: %#v", res["status"])
	}
	if res["title"] != "t" || res["currency"] != "TWD" {
		t.Fatalf("unexpected core: title=%v currency=%v", res["title"], res["currency"])
	}
	if res["price"] != "123" {
		t.Fatalf("unexpected price: %#v", res["price"])
	}

	images, ok := res["images"].([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("unexpected images: %#v", res["images"])
	}

	vars, ok := res["variations"].([]Variation)
	if !ok || len(vars) != 2 {
		t.Fatalf("unexpected variations: %#v", res["variations"])
	}
	if vars[0].Title != "v0" || vars[0].Image != "https://img/v0" {
		t.Fatalf("unexpected mapping: %#v", vars[0])
	}
	if vars[1].Title != "v1" || vars[1].Image != "" {
		t.Fatalf("unexpected mapping: %#v", vars[1])
	}
}

func TestBuildCrawlResultFromSnapshot_OK_ArrayArtifactFormats(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeJSON(t, filepath.Join(dir, "page_state.json"), map[string]any{
		"url": "https://example.com/p/1",
		"extracted": map[string]any{
			"title":       "t",
			"description": "d",
			"currency":    "TWD",
			"price":       "123",
		},
	})

	// Array formats (as produced by the skill in some runs).
	writeJSON(t, filepath.Join(dir, "overlay_images.json"), []string{"https://img/1", "https://img/2"})
	writeJSON(t, filepath.Join(dir, "variations.json"), []map[string]any{
		{"position": 0, "text": "v0"},
		{"position": 1, "text": "v1"},
	})
	writeJSON(t, filepath.Join(dir, "variation_image_map.json"), []map[string]any{
		{"variation": "v0", "image": "https://img/v0"},
	})

	ptr := SnapshotPointer{
		URL:         "https://example.com/p/1",
		Status:      "ok",
		CapturedAt:  "2026-01-01T00:00:00Z",
		RunID:       "r1",
		ArtifactDir: dir,
		SnapshotFiles: SnapshotFiles{
			PageState:         "page_state.json",
			OverlayImages:     "overlay_images.json",
			Variations:        "variations.json",
			VariationImageMap: "variation_image_map.json",
		},
	}

	res, err := buildCrawlResultFromSnapshot(ptr, "")
	if err != nil {
		t.Fatalf("buildCrawlResultFromSnapshot error: %v", err)
	}

	images, ok := res["images"].([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("unexpected images: %#v", res["images"])
	}

	vars, ok := res["variations"].([]Variation)
	if !ok || len(vars) != 2 {
		t.Fatalf("unexpected variations: %#v", res["variations"])
	}
	if vars[0].Title != "v0" || vars[0].Image != "https://img/v0" {
		t.Fatalf("unexpected mapping: %#v", vars[0])
	}
}

func TestSnapshotPointer_filePath_DoesNotDoublePrefixArtifactDir(t *testing.T) {
	t.Parallel()

	ptr := SnapshotPointer{
		RunID:       "r1",
		ArtifactDir: filepath.Join("out", "artifacts", "r1"),
		SnapshotFiles: SnapshotFiles{
			PageState: filepath.Join("out", "artifacts", "r1", "page_state.json"),
		},
	}

	got := ptr.filePath(ptr.SnapshotFiles.PageState)
	want := filepath.Join("out", "artifacts", "r1", "page_state.json")
	if got != want {
		t.Fatalf("unexpected filePath: got=%q want=%q", got, want)
	}
}

func TestExtractCoreFromPageStateJSON_SanitizesControlCharsInStrings(t *testing.T) {
	t.Parallel()

	// NOTE: This JSON is intentionally invalid: it contains literal CR/LF inside a string literal.
	raw := []byte("{\n" +
		`  "extracted": { "title": "t", "description": "d", "currency": "TWD", "price": "1" },` + "\n" +
		"  \"meta\": { \"og:description\": \"line1\r\nline2\" }\n" +
		"}\n")

	got, err := extractCoreFromPageStateJSON(raw)
	if err != nil {
		t.Fatalf("extractCoreFromPageStateJSON error: %v", err)
	}
	if got.Title != "t" || got.Currency != "TWD" || got.Price != "1" {
		t.Fatalf("unexpected core: %#v", got)
	}
}

func TestExtractCoreFromPageStateJSON_FallsBackToExtractedSubstring(t *testing.T) {
	t.Parallel()

	// Intentionally invalid JSON overall: jsonld_raw strings contain unescaped quotes and meta contains raw newlines.
	// The fallback should still find and parse the extracted object.
	raw := []byte("{\n" +
		"  \"meta\": { \"description\": \"line1\r\nline2\" },\n" +
		"  \"jsonld_raw\": [\n" +
		"    \"{\"@context\":\"http://schema.org\"}\",\n" +
		"    \"{\"@type\":\"Product\"}\"\n" +
		"  ],\n" +
		"  \"extracted\": { \"title\": \"t\", \"description\": \"d\", \"currency\": \"TWD\", \"price\": \"1\" }\n" +
		"}\n")

	got, err := extractCoreFromPageStateJSON(raw)
	if err != nil {
		t.Fatalf("extractCoreFromPageStateJSON error: %v", err)
	}
	if got.Title != "t" || got.Description != "d" || got.Currency != "TWD" || got.Price != "1" {
		t.Fatalf("unexpected core: %#v", got)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
