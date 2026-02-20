package runner

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// CrawlOut matches the shared crawler output contract used by orchestrator skills.
// The runner may add extra keys to the persisted output, but the fields below
// are the ones we validate strictly.
type CrawlOut struct {
	URL        string `json:"url" validate:"required"`
	Status     string `json:"status" validate:"required,oneof=ok needs_manual error"`
	CapturedAt string `json:"captured_at" validate:"required,captured_at"`

	Notes string `json:"notes,omitempty" validate:"required_if=Status needs_manual"`
	Error string `json:"error,omitempty" validate:"required_if=Status error"`

	// Core fields are optional to allow degraded outputs when extraction fails.
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	Currency    string      `json:"currency,omitempty"`
	Price       any         `json:"price,omitempty" validate:"omitempty,price"`
	Images      []any       `json:"images,omitempty"`
	Variations  []Variation `json:"variations,omitempty" validate:"omitempty,dive"`
}

type Variation struct {
	Title    string `json:"title" validate:"required"`
	Position int    `json:"position" validate:"min=0"`
	// Phase 1 compatibility: accept both legacy `image` and new `images`.
	Image  string   `json:"image,omitempty"`
	Images []string `json:"images,omitempty"`
}

func validateCrawlOut(out CrawlOut) error {
	v := validator.New()

	if err := v.RegisterValidation("captured_at", validateCapturedAt); err != nil {
		return err
	}
	if err := v.RegisterValidation("price", validatePrice); err != nil {
		return err
	}

	if err := v.Struct(out); err != nil {
		return fmt.Errorf("output contract validation failed: %w", err)
	}
	return nil
}

func validateCapturedAt(fl validator.FieldLevel) bool {
	s, ok := fl.Field().Interface().(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, s); err == nil {
		return true
	}
	return false
}

var numericPriceRE = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

func validatePrice(fl validator.FieldLevel) bool {
	// We validate the "price" field only when Status=ok (enforced by required_if).
	// Accept:
	// - json.Number (when decoder.UseNumber is used)
	// - float64/int/etc (when unmarshaled without UseNumber)
	// - numeric strings (e.g. "123", "123.45")
	v := fl.Field().Interface()
	switch vv := v.(type) {
	case nil:
		// Allow nil when price is not required (i.e., Status != ok). When Status=ok,
		// required_if will fail separately.
		return true
	case json.Number:
		s := strings.TrimSpace(vv.String())
		return s != "" && numericPriceRE.MatchString(s)
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return true
	case string:
		s := strings.TrimSpace(vv)
		return s != "" && numericPriceRE.MatchString(s)
	default:
		return false
	}
}
