# Ensure Gemini CLI results are valid JSON (Go runner guide)

This guide explains how to modify your existing `GeminiRunner` so the final return value is always a **single valid JSON object** that matches your crawler output contract.

## 1) Background: what Gemini CLI JSON mode guarantees

When Gemini CLI runs in headless mode with JSON output (`-o json` / `--output-format json`), stdout is a **JSON wrapper** designed for programmatic processing. The wrapper includes the model's output in a field commonly named `response`, plus statistics and metadata. The wrapper being JSON is useful, but it does **not** guarantee that the model text inside `response` is itself valid JSON.

In practice, the model may still:
- wrap JSON in Markdown fences (```json ... ```),
- add extra text before or after the JSON,
- output malformed JSON,
- output multiple JSON objects.

So you need to enforce JSON correctness in your Go code.

## 2) Target behavior

Your runner should return:
- Exactly one JSON object string (no Markdown).
- JSON must match your contract (required fields depend on `status`).
- `price` may be a JSON number or a numeric string.

## 3) Recommended solution (CLI-compatible)

### Summary

1. Keep `-o json` (wrapper JSON).
2. Unwrap wrapper to get the model text (`response`).
3. Extract the first valid JSON object from that text using a real JSON parser (not substring slicing).
4. Unmarshal into a Go struct and validate required fields.
5. If extraction or validation fails, do exactly **one** repair pass by asking Gemini to output valid JSON only.
6. Return canonical JSON by re-marshaling the struct.

This yields strong reliability while staying on Gemini CLI.

## 4) Implementation steps

### Step A: Harden the prompt

At the end of your existing Shopee extraction prompt, append something like:

- "Return JSON ONLY. No markdown fences. No extra text."
- "Output must be a single JSON object matching the contract exactly."
- "If blocked or missing required data, return `status=needs_manual` or `status=error` but still output valid JSON."

This reduces failures but is not sufficient on its own.

### Step B: Replace fragile string slicing with a decoder-based extractor

Your current `sanitizeGeminiResponse()` uses "first '{' to last '}'" slicing, which can break if braces appear inside strings or if there are multiple objects.

Add a decoder-based extractor:

```go
func extractFirstJSONObject(raw string) (string, error) {
    raw = strings.TrimSpace(raw)
    if raw == "" {
        return "", fmt.Errorf("empty response")
    }

    // Handle markdown fences like ```json ... ```
    if strings.HasPrefix(raw, "```") {
        if fenced := extractFirstMarkdownFence(raw); fenced != "" {
            raw = strings.TrimSpace(fenced)
        }
    }

    // Scan for a syntactically valid JSON object starting at each '{'.
    for i := 0; i < len(raw); i++ {
        if raw[i] != '{' {
            continue
        }

        dec := json.NewDecoder(strings.NewReader(raw[i:]))
        dec.UseNumber()

        var v any
        if err := dec.Decode(&v); err != nil {
            continue
        }

        // Ensure top-level is an object
        if _, ok := v.(map[string]any); !ok {
            continue
        }

        b, err := json.Marshal(v)
        if err != nil {
            return "", err
        }
        return string(b), nil
    }

    return "", fmt.Errorf("no valid JSON object found")
}
```

This:
- tolerates leading/trailing chatter,
- tolerates markdown fences,
- avoids being confused by braces inside JSON strings,
- returns canonical JSON.

### Step C: Validate output against your contract

Define a struct and explicit validation rules.

```go
type CrawlOut struct {
    URL         string `json:"url"`
    Status      string `json:"status"`
    CapturedAt  string `json:"captured_at"`

    Notes       string `json:"notes,omitempty"`
    Error       string `json:"error,omitempty"`

    Title       string `json:"title,omitempty"`
    Description string `json:"description,omitempty"`
    Currency    string `json:"currency,omitempty"`

    Price       any    `json:"price,omitempty"` // number or numeric string
}

func validateCrawlOut(out CrawlOut) error {
    if strings.TrimSpace(out.URL) == "" {
        return fmt.Errorf("missing url")
    }
    if strings.TrimSpace(out.Status) == "" {
        return fmt.Errorf("missing status")
    }
    if strings.TrimSpace(out.CapturedAt) == "" {
        return fmt.Errorf("missing captured_at")
    }

    switch out.Status {
    case "ok":
        if strings.TrimSpace(out.Title) == "" {
            return fmt.Errorf("status=ok missing title")
        }
        if strings.TrimSpace(out.Description) == "" {
            return fmt.Errorf("status=ok missing description")
        }
        if strings.TrimSpace(out.Currency) == "" {
            return fmt.Errorf("status=ok missing currency")
        }
        if out.Price == nil {
            return fmt.Errorf("status=ok missing price")
        }
    case "needs_manual":
        if strings.TrimSpace(out.Notes) == "" {
            return fmt.Errorf("status=needs_manual missing notes")
        }
    case "error":
        if strings.TrimSpace(out.Error) == "" {
            return fmt.Errorf("status=error missing error")
        }
    default:
        return fmt.Errorf("invalid status: %s", out.Status)
    }

    return nil
}
```

Optional: validate that `price` is numeric. Accept `json.Number`, `float64`, `int`, or a string matching `^[0-9]+(\.[0-9]+)?$`.

### Step D: Add a single repair pass

If extraction, unmarshal, or validation fails, do exactly **one** additional Gemini call to convert the bad output into valid JSON.

Repair prompt template:

```text
You returned invalid JSON.

Convert the TEXT below into EXACTLY ONE valid JSON object matching this contract:
- Keys: url, status, captured_at, notes, error, title, description, currency, price
- status must be one of: ok | needs_manual | error
- If required fields are missing, set status="error" and explain in error.
- Output JSON ONLY. No markdown. No extra text.

TEXT:
<<<
{previous_output_here}
>>>
```

Implementation notes:
- Prevent infinite recursion by tracking an `attempt` or `allowRepair` flag.
- Consider restricting tool use during repair by telling the model: "Do not call tools".

### Step E: Return canonical JSON

Even if Gemini returns valid JSON, re-marshal your struct so formatting is canonical:

```go
b, _ := json.Marshal(out)
return string(b), nil
```

### Step F: Add tests

Add unit tests for:
- JSON inside markdown fences
- extra text before/after JSON
- multiple JSON objects (ensure the first valid object is extracted)
- braces inside JSON strings
- missing required fields for `status=ok`

## 5) Alternative: strongest guarantee (Gemini API structured outputs)

If you need a hard guarantee that the model output is valid JSON matching a schema (no repair loop), use Gemini API structured outputs:
- set `response_mime_type` to `application/json`
- provide a JSON Schema for the response

This enforces JSON at the model generation layer rather than post-processing.

## 6) Minimal patch checklist

- [ ] Keep `-o json` and `unwrapGeminiJSON()`.
- [ ] Add `extractFirstJSONObject()` (decoder-based).
- [ ] Add `CrawlOut` struct + `validateCrawlOut()`.
- [ ] Update `Run()` flow to: unwrap -> extract -> unmarshal -> validate -> marshal.
- [ ] Add a one-time repair pass on failure.
- [ ] Add tests and logs for repair attempts.
