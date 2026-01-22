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

