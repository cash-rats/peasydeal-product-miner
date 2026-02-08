package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const shopeePageSnapshotSkill = "shopee-page-snapshot"

type SnapshotPointer struct {
	URL        string `json:"url"`
	Status     string `json:"status"`
	CapturedAt string `json:"captured_at"`

	RunID       string `json:"run_id"`
	ArtifactDir string `json:"artifact_dir"`

	SnapshotFiles SnapshotFiles `json:"snapshot_files"`

	Notes string `json:"notes,omitempty"`
	Error string `json:"error,omitempty"`
}

type SnapshotFiles struct {
	Snapshot          string `json:"snapshot"`
	PageHTML          string `json:"page_html"`
	PageState         string `json:"page_state"`
	OverlayImages     string `json:"overlay_images"`
	Variations        string `json:"variations"`
	VariationImageMap string `json:"variation_image_map"`
}

func parseSnapshotPointer(raw string) (SnapshotPointer, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SnapshotPointer{}, fmt.Errorf("empty snapshot pointer")
	}

	// Try to extract the first JSON object (in case the tool accidentally emits extra text).
	extracted, err := extractFirstJSONObject(raw)
	if err != nil {
		extracted = raw
	}

	dec := json.NewDecoder(strings.NewReader(extracted))
	dec.UseNumber()

	var ptr SnapshotPointer
	if err := dec.Decode(&ptr); err != nil {
		return SnapshotPointer{}, fmt.Errorf("invalid snapshot pointer JSON: %w", err)
	}

	if strings.TrimSpace(ptr.URL) == "" {
		return SnapshotPointer{}, fmt.Errorf("snapshot pointer missing url")
	}
	if strings.TrimSpace(ptr.Status) == "" {
		return SnapshotPointer{}, fmt.Errorf("snapshot pointer missing status")
	}
	if strings.TrimSpace(ptr.ArtifactDir) == "" {
		// Be tolerant: some LLM runs may omit artifact_dir even though run_id and/or snapshot_files are present.
		// Try to infer it from run_id or absolute snapshot file paths.
		if inferred := inferArtifactDir(ptr); inferred != "" {
			ptr.ArtifactDir = inferred
		} else {
			// Allow missing ArtifactDir only if snapshot files are absolute paths (caller can still resolve).
			if !snapshotFilesAllEmpty(ptr.SnapshotFiles) && snapshotFilesAnyAbs(ptr.SnapshotFiles) {
				return ptr, nil
			}
			return SnapshotPointer{}, fmt.Errorf("snapshot pointer missing artifact_dir")
		}
	}

	return ptr, nil
}

func snapshotFilesAllEmpty(f SnapshotFiles) bool {
	return strings.TrimSpace(f.Snapshot) == "" &&
		strings.TrimSpace(f.PageHTML) == "" &&
		strings.TrimSpace(f.PageState) == "" &&
		strings.TrimSpace(f.OverlayImages) == "" &&
		strings.TrimSpace(f.Variations) == "" &&
		strings.TrimSpace(f.VariationImageMap) == ""
}

func snapshotFilesAnyAbs(f SnapshotFiles) bool {
	for _, p := range []string{f.Snapshot, f.PageHTML, f.PageState, f.OverlayImages, f.Variations, f.VariationImageMap} {
		if filepath.IsAbs(strings.TrimSpace(p)) {
			return true
		}
	}
	return false
}

func inferArtifactDir(ptr SnapshotPointer) string {
	// 1) If any snapshot file path is absolute, use its directory.
	for _, p := range []string{
		ptr.SnapshotFiles.PageState,
		ptr.SnapshotFiles.Snapshot,
		ptr.SnapshotFiles.PageHTML,
		ptr.SnapshotFiles.OverlayImages,
		ptr.SnapshotFiles.Variations,
		ptr.SnapshotFiles.VariationImageMap,
	} {
		p = strings.TrimSpace(p)
		if p != "" && filepath.IsAbs(p) {
			return filepath.Dir(p)
		}
	}

	// 2) If run_id is present, try common artifact roots (host + docker mount).
	runID := strings.TrimSpace(ptr.RunID)
	if runID == "" {
		return ""
	}

	candidates := []string{
		filepath.Join("out", "artifacts", runID),                             // host + repo-relative
		filepath.Join(string(filepath.Separator), "out", "artifacts", runID), // container /out mount
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c
		}
	}

	// Best-effort fallback even if it doesn't exist yet.
	return candidates[0]
}

func (p SnapshotPointer) filePath(relOrAbs string) string {
	s := strings.TrimSpace(relOrAbs)
	if s == "" {
		return ""
	}
	if filepath.IsAbs(s) {
		return s
	}
	artifactDir := strings.TrimSpace(p.ArtifactDir)
	if artifactDir == "" {
		return s
	}

	// Be tolerant: some skills may emit snapshot_files paths that already include artifact_dir
	// (e.g. "out/artifacts/<run_id>/page_state.json") while also setting artifact_dir to the same
	// directory. Avoid duplicating the prefix.
	cleanArtifactDir := filepath.Clean(artifactDir)
	cleanS := filepath.Clean(s)
	sep := string(filepath.Separator)
	if cleanS == cleanArtifactDir || strings.HasPrefix(cleanS, cleanArtifactDir+sep) {
		return cleanS
	}

	return filepath.Join(cleanArtifactDir, cleanS)
}

func buildCrawlResultFromSnapshot(ptr SnapshotPointer, fallbackURL string) (Result, error) {
	url := strings.TrimSpace(ptr.URL)
	if url == "" {
		url = strings.TrimSpace(fallbackURL)
	}

	out := Result{
		"url":         url,
		"captured_at": ptr.CapturedAt,
		"status":      strings.TrimSpace(ptr.Status),
	}

	// Ensure contract-required timestamp always exists.
	if strings.TrimSpace(ptr.CapturedAt) == "" {
		out["captured_at"] = nowISO()
	}

	switch strings.TrimSpace(ptr.Status) {
	case "needs_manual":
		out["notes"] = strings.TrimSpace(ptr.Notes)
		if strings.TrimSpace(out["notes"].(string)) == "" {
			out["notes"] = "blocked or requires manual intervention"
		}
		out["images"] = []any{}
		out["variations"] = []any{}
		return out, nil
	case "error":
		out["error"] = strings.TrimSpace(ptr.Error)
		if strings.TrimSpace(out["error"].(string)) == "" {
			out["error"] = "snapshot failed"
		}
		out["images"] = []any{}
		out["variations"] = []any{}
		return out, nil
	case "ok":
		// continue
	default:
		return nil, fmt.Errorf("unsupported snapshot status: %q", ptr.Status)
	}

	// 1) Core fields from page_state.json.
	pageStatePath := ptr.filePath(ptr.SnapshotFiles.PageState)
	if pageStatePath == "" {
		return nil, fmt.Errorf("missing snapshot_files.page_state")
	}
	pageStateBytes, err := os.ReadFile(pageStatePath)
	if err != nil {
		return nil, fmt.Errorf("read page_state: %w", err)
	}

	core, err := extractCoreFromPageStateJSON(pageStateBytes)
	if err != nil {
		return nil, err
	}
	out["title"] = core.Title
	out["description"] = core.Description
	out["currency"] = core.Currency
	out["price"] = core.Price

	// 2) Images from overlay_images.json (optional).
	imagesPath := ptr.filePath(ptr.SnapshotFiles.OverlayImages)
	if imagesPath != "" {
		if b, rerr := os.ReadFile(imagesPath); rerr == nil {
			images := extractImagesFromOverlayJSON(b)
			out["images"] = images
		}
	}
	if _, ok := out["images"]; !ok {
		out["images"] = []any{}
	}

	// 3) Variations from variations.json + mapping from variation_image_map.json (optional).
	var variations []Variation
	variationsPath := ptr.filePath(ptr.SnapshotFiles.Variations)
	if variationsPath != "" {
		if b, rerr := os.ReadFile(variationsPath); rerr == nil {
			variations = extractVariationsFromJSON(b)
		}
	}

	mappingPath := ptr.filePath(ptr.SnapshotFiles.VariationImageMap)
	if mappingPath != "" && len(variations) > 0 {
		if b, rerr := os.ReadFile(mappingPath); rerr == nil {
			imgByText := extractVariationImageMapFromJSON(b)
			for i := range variations {
				if variations[i].Image != "" {
					continue
				}
				if u, ok := imgByText[variations[i].Title]; ok {
					variations[i].Image = u
				}
			}
		}
	}

	if len(variations) > 0 {
		out["variations"] = variations
	} else {
		out["variations"] = []any{}
	}

	return out, nil
}

type coreFields struct {
	Title       string
	Description string
	Currency    string
	Price       any
}

func extractCoreFromPageStateJSON(b []byte) (coreFields, error) {
	obj, err := decodeJSONToMap(b)
	if err != nil {
		// Some LLM-written artifacts accidentally embed raw control characters (CR/LF/TAB) inside JSON
		// string literals (e.g. meta descriptions with line breaks). That's invalid JSON; sanitize and retry.
		sanitized := sanitizeJSONControlCharsInStrings(b)
		if !bytes.Equal(sanitized, b) {
			if obj2, err2 := decodeJSONToMap(sanitized); err2 == nil {
				obj = obj2
				err = nil
			} else {
				err = err2
			}
		}
		if err != nil {
			// If the artifact is not parseable as a whole (e.g. jsonld_raw contains unescaped quotes),
			// salvage the required core fields by extracting the `extracted` object substring.
			if core, ok := extractCoreFromPageStateTextFallback(b); ok {
				return core, nil
			}
			return coreFields{}, fmt.Errorf("invalid page_state.json (must be JSON): %w", err)
		}
	}

	// Preferred: page_state.extracted.{title,description,currency,price}
	if extracted, ok := obj["extracted"].(map[string]any); ok {
		c := coreFields{
			Title:       strings.TrimSpace(asString(extracted["title"])),
			Description: strings.TrimSpace(asString(extracted["description"])),
			Currency:    strings.TrimSpace(asString(extracted["currency"])),
			Price:       extracted["price"],
		}
		if c.Title != "" && c.Description != "" && c.Currency != "" && !isEmptyPrice(c.Price) {
			return c, nil
		}
	}

	// Fallback: top-level keys (useful if the snapshot writer is simplified).
	c := coreFields{
		Title:       strings.TrimSpace(asString(obj["title"])),
		Description: strings.TrimSpace(asString(obj["description"])),
		Currency:    strings.TrimSpace(asString(obj["currency"])),
		Price:       obj["price"],
	}
	if c.Title != "" && c.Description != "" && c.Currency != "" && !isEmptyPrice(c.Price) {
		return c, nil
	}

	return coreFields{}, fmt.Errorf("page_state.json missing extracted core fields (title/description/currency/price)")
}

func extractCoreFromPageStateTextFallback(b []byte) (coreFields, bool) {
	s := string(b)
	idx := strings.Index(s, `"extracted"`)
	if idx < 0 {
		return coreFields{}, false
	}

	// Find the first '{' after the "extracted" key's ':'.
	colon := strings.Index(s[idx:], ":")
	if colon < 0 {
		return coreFields{}, false
	}
	startSearch := idx + colon + 1
	open := strings.Index(s[startSearch:], "{")
	if open < 0 {
		return coreFields{}, false
	}
	start := startSearch + open

	end := findMatchingJSONObjectEnd([]byte(s), start)
	if end <= start {
		return coreFields{}, false
	}

	segment := []byte(s[start:end])
	segment = sanitizeJSONControlCharsInStrings(segment)
	obj, err := decodeJSONToMap(segment)
	if err != nil {
		return coreFields{}, false
	}

	c := coreFields{
		Title:       strings.TrimSpace(asString(obj["title"])),
		Description: strings.TrimSpace(asString(obj["description"])),
		Currency:    strings.TrimSpace(asString(obj["currency"])),
		Price:       obj["price"],
	}
	if c.Title != "" && c.Description != "" && c.Currency != "" && !isEmptyPrice(c.Price) {
		return c, true
	}
	return coreFields{}, false
}

func findMatchingJSONObjectEnd(b []byte, start int) int {
	if start < 0 || start >= len(b) || b[start] != '{' {
		return -1
	}
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(b); i++ {
		c := b[i]

		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func decodeJSONToMap(b []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()

	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// sanitizeJSONControlCharsInStrings escapes raw control characters (< 0x20) that appear inside JSON string
// literals. This makes the JSON valid while preserving the original content as closely as possible.
//
// This does NOT attempt to repair structurally-invalid JSON; it's only meant to handle cases like:
//
//	"meta": { "og:description": "line1\r\nline2" }   (with literal CR/LF)
func sanitizeJSONControlCharsInStrings(in []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(in) + 64)

	inString := false
	escaped := false

	for i := 0; i < len(in); i++ {
		c := in[i]

		if !inString {
			out.WriteByte(c)
			if c == '"' {
				inString = true
				escaped = false
			}
			continue
		}

		// inString
		if escaped {
			out.WriteByte(c)
			escaped = false
			continue
		}

		switch c {
		case '\\':
			out.WriteByte(c)
			escaped = true
		case '"':
			out.WriteByte(c)
			inString = false
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		default:
			if c < 0x20 {
				// Other control chars -> \u00XX
				const hex = "0123456789abcdef"
				out.WriteString(`\u00`)
				out.WriteByte(hex[c>>4])
				out.WriteByte(hex[c&0x0f])
			} else {
				out.WriteByte(c)
			}
		}
	}

	return out.Bytes()
}

func asString(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case json.Number:
		return vv.String()
	default:
		return ""
	}
}

func isEmptyPrice(v any) bool {
	switch vv := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(vv) == ""
	case json.Number:
		return strings.TrimSpace(vv.String()) == ""
	default:
		// numeric types are fine
		return false
	}
}

func extractImagesFromOverlayJSON(b []byte) []any {
	// Accept both formats:
	// 1) { "images": ["..."] }
	// 2) ["..."]
	var urls []string

	var obj struct {
		Images []string `json:"images"`
	}
	if err := json.Unmarshal(b, &obj); err == nil && len(obj.Images) > 0 {
		urls = obj.Images
	} else {
		var arr []string
		if err := json.Unmarshal(b, &arr); err == nil && len(arr) > 0 {
			urls = arr
		} else {
			return []any{}
		}
	}

	out := make([]any, 0, len(urls))
	seen := make(map[string]bool, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func extractVariationsFromJSON(b []byte) []Variation {
	// Accept the snapshot format:
	// { "variations": [ { "position": 0, "text": "..." }, ... ] }
	// and also:
	// [ { "position": 0, "text": "..." }, ... ]
	type rawVar struct {
		Position int    `json:"position"`
		Text     string `json:"text"`
	}

	var vars []rawVar
	var wrapped struct {
		Variations []rawVar `json:"variations"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && len(wrapped.Variations) > 0 {
		vars = wrapped.Variations
	} else {
		if err := json.Unmarshal(b, &vars); err != nil {
			return nil
		}
	}

	out := make([]Variation, 0, len(vars))
	for _, v := range vars {
		title := strings.TrimSpace(v.Text)
		if title == "" {
			continue
		}
		out = append(out, Variation{
			Title:    title,
			Position: v.Position,
		})
	}
	return out
}

func extractVariationImageMapFromJSON(b []byte) map[string]string {
	// Accept the snapshot format:
	// { "map": [ { "text": "...", "imageUrl": "..." }, ... ] }
	// and also:
	// [ { "variation": "...", "image": "..." }, ... ]
	type rawMapItem struct {
		Text      string `json:"text"`
		ImageURL  string `json:"imageUrl"`
		Variation string `json:"variation"`
		Image     string `json:"image"`
	}

	var items []rawMapItem
	var wrapped struct {
		Map []rawMapItem `json:"map"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil && len(wrapped.Map) > 0 {
		items = wrapped.Map
	} else {
		if err := json.Unmarshal(b, &items); err != nil {
			return map[string]string{}
		}
	}

	out := make(map[string]string, len(items))
	for _, m := range items {
		k := strings.TrimSpace(m.Text)
		v := strings.TrimSpace(m.ImageURL)
		if k == "" {
			k = strings.TrimSpace(m.Variation)
		}
		if v == "" {
			v = strings.TrimSpace(m.Image)
		}
		if k == "" || v == "" {
			continue
		}
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}
	return out
}
