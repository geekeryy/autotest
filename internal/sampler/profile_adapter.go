package sampler

import (
	"math/rand/v2"
	"strings"

	"autotest/internal/testprofile"
)

// FieldProfileAdapter adapts a slice of testprofile.FieldProfile into the
// ProfileLookup interface so the sampler can use historically observed values.
type FieldProfileAdapter struct {
	// index maps normalised "location.fieldPath" → observed values.
	index map[string][]any
}

// NewFieldProfileAdapter creates a ProfileLookup that serves observed values
// from the given FieldProfile slice. Only profiles matching method+path are
// included; pass empty strings to match all.
func NewFieldProfileAdapter(method, path string, profiles []testprofile.FieldProfile) *FieldProfileAdapter {
	idx := map[string][]any{}
	for _, fp := range profiles {
		if method != "" && !strings.EqualFold(fp.Method, method) {
			continue
		}
		if path != "" && fp.Path != path {
			continue
		}
		if len(fp.ObservedValues) == 0 {
			continue
		}
		key := fp.Location + "." + fp.FieldPath
		idx[key] = append(idx[key], fp.ObservedValues...)
	}
	return &FieldProfileAdapter{index: idx}
}

// Lookup returns historically observed values for the given body field path.
// It searches for "body.<fieldPath>" in the index.
func (a *FieldProfileAdapter) Lookup(fieldPath string) []any {
	if a == nil || len(a.index) == 0 {
		return nil
	}
	// Try exact match on body location first.
	key := "body." + fieldPath
	if vals, ok := a.index[key]; ok && len(vals) > 0 {
		return vals
	}
	// Also try query and header locations for non-body params.
	key = "query." + fieldPath
	if vals, ok := a.index[key]; ok && len(vals) > 0 {
		return vals
	}
	key = "headers." + fieldPath
	if vals, ok := a.index[key]; ok && len(vals) > 0 {
		return vals
	}
	return nil
}

// PickRandomString picks a random string value from the observed values.
// Non-string values are ignored. Returns empty string if no string value found.
func PickRandomString(values []any) string {
	var strs []string
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			strs = append(strs, s)
		}
	}
	if len(strs) == 0 {
		return ""
	}
	return strs[rand.N(len(strs))]
}
