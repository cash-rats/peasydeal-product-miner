package runner

import "testing"

func TestResultEnsureImagesArray_DefaultsWhenMissing(t *testing.T) {
	r := Result{
		"url":    "https://example.com",
		"status": "ok",
	}

	r.ensureImagesArray()

	v, ok := r["images"]
	if !ok {
		t.Fatalf("expected images key to be set")
	}

	images, ok := v.([]any)
	if !ok {
		t.Fatalf("expected images to be []any, got %T", v)
	}
	if len(images) != 0 {
		t.Fatalf("expected empty images array, got len=%d", len(images))
	}
}

func TestResultEnsureImagesArray_DefaultsWhenNil(t *testing.T) {
	r := Result{
		"url":    "https://example.com",
		"status": "ok",
		"images": nil,
	}

	r.ensureImagesArray()

	images, ok := r["images"].([]any)
	if !ok {
		t.Fatalf("expected images to be []any, got %T", r["images"])
	}
	if len(images) != 0 {
		t.Fatalf("expected empty images array, got len=%d", len(images))
	}
}

func TestNormalizeResult_VariationsLegacyImageBackfilledToImages(t *testing.T) {
	r := Result{
		"url":    "https://example.com",
		"status": "ok",
		"variations": []any{
			map[string]any{
				"title":    "v0",
				"position": 0,
				"image":    "https://img/v0",
			},
		},
	}

	normalizeResult(r)

	vars, ok := r["variations"].([]any)
	if !ok || len(vars) != 1 {
		t.Fatalf("unexpected variations: %#v", r["variations"])
	}
	obj, ok := vars[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected variation object: %#v", vars[0])
	}
	images, ok := obj["images"].([]string)
	if !ok {
		t.Fatalf("expected []string images, got %#v", obj["images"])
	}
	if len(images) != 1 || images[0] != "https://img/v0" {
		t.Fatalf("unexpected images: %#v", images)
	}
	if obj["image"] != "https://img/v0" {
		t.Fatalf("unexpected image: %#v", obj["image"])
	}
}

func TestNormalizeResult_VariationsImagesDedupeAndCanonicalImage(t *testing.T) {
	r := Result{
		"url":    "https://example.com",
		"status": "ok",
		"variations": []any{
			map[string]any{
				"title":    "v0",
				"position": 0,
				"image":    "https://img/v0-2",
				"images":   []any{"https://img/v0-1", "https://img/v0-1", "https://img/v0-2"},
			},
		},
	}

	normalizeResult(r)

	vars := r["variations"].([]any)
	obj := vars[0].(map[string]any)
	images := obj["images"].([]string)
	if len(images) != 2 || images[0] != "https://img/v0-1" || images[1] != "https://img/v0-2" {
		t.Fatalf("unexpected images: %#v", images)
	}
	if obj["image"] != "https://img/v0-1" {
		t.Fatalf("unexpected canonical image: %#v", obj["image"])
	}
}
