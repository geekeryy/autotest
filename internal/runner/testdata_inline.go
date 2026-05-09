package runner

import (
	"autotest/internal/testdata"
)

// collectInlineTestDataReferences scans every string field of the request
// definition for `{{$ds.*}}` references and returns the parsed list. It mirrors
// the behaviour of `collectInlineSQLReferences`.
func collectInlineTestDataReferences(def RequestDefinition) ([]testdata.InlineReference, error) {
	var refs []testdata.InlineReference
	var err error
	appendFromString := func(input string) bool {
		var parsed []testdata.InlineReference
		parsed, err = testdata.ParseInlineReferences(input)
		if err != nil {
			return false
		}
		refs = append(refs, parsed...)
		return true
	}

	for _, value := range []string{def.Path, def.URL, def.Method} {
		if !appendFromString(value) {
			return nil, err
		}
	}
	for key, value := range def.Query {
		if !appendFromString(key) || !appendFromString(value) {
			return nil, err
		}
	}
	for key, value := range def.Headers {
		if !appendFromString(key) || !appendFromString(value) {
			return nil, err
		}
	}
	for key, value := range stringifyVariables(def.Variables) {
		if !appendFromString(key) || !appendFromString(value) {
			return nil, err
		}
	}
	if !collectInlineTestDataReferencesFromAny(def.Body, &refs, &err) {
		return nil, err
	}
	return refs, nil
}

// collectInlineTestDataReferencesFromVariables scans a freeform variables map
// (e.g. caller-provided overrides) for `{{$ds.*}}` references.
func collectInlineTestDataReferencesFromVariables(vars map[string]any) ([]testdata.InlineReference, error) {
	if len(vars) == 0 {
		return nil, nil
	}
	var refs []testdata.InlineReference
	var err error
	if !collectInlineTestDataReferencesFromAny(vars, &refs, &err) {
		return nil, err
	}
	return refs, nil
}

func collectInlineTestDataReferencesFromAny(input any, refs *[]testdata.InlineReference, err *error) bool {
	switch value := input.(type) {
	case string:
		var parsed []testdata.InlineReference
		parsed, *err = testdata.ParseInlineReferences(value)
		if *err != nil {
			return false
		}
		*refs = append(*refs, parsed...)
	case []any:
		for _, item := range value {
			if !collectInlineTestDataReferencesFromAny(item, refs, err) {
				return false
			}
		}
	case map[string]any:
		for key, item := range value {
			var parsed []testdata.InlineReference
			parsed, *err = testdata.ParseInlineReferences(key)
			if *err != nil {
				return false
			}
			*refs = append(*refs, parsed...)
			if !collectInlineTestDataReferencesFromAny(item, refs, err) {
				return false
			}
		}
	case map[string]string:
		for key, item := range value {
			var parsed []testdata.InlineReference
			parsed, *err = testdata.ParseInlineReferences(key)
			if *err != nil {
				return false
			}
			*refs = append(*refs, parsed...)
			parsed, *err = testdata.ParseInlineReferences(item)
			if *err != nil {
				return false
			}
			*refs = append(*refs, parsed...)
		}
	}
	return true
}
