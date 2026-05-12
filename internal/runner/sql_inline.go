package runner

import (
	"fmt"

	"autotest/internal/paramsource"
	"autotest/internal/templating"
)

func collectInlineSQLReferences(def RequestDefinition) ([]paramsource.InlineReference, error) {
	var refs []paramsource.InlineReference
	var err error
	appendFromString := func(input string) bool {
		var parsed []paramsource.InlineReference
		parsed, err = parseInlineSQLReferences(input)
		if err != nil {
			return false
		}
		refs = append(refs, parsed...)
		return true
	}

	for _, value := range []string{def.Path, def.URL} {
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
	if !collectInlineSQLReferencesFromAny(def.Body, &refs, &err) {
		return nil, err
	}
	return refs, nil
}

func stringifyVariables(input map[string]any) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = variableString(value)
	}
	return out
}

func collectInlineSQLReferencesFromVariables(vars map[string]any) ([]paramsource.InlineReference, error) {
	if len(vars) == 0 {
		return nil, nil
	}
	var refs []paramsource.InlineReference
	var err error
	if !collectInlineSQLReferencesFromAny(vars, &refs, &err) {
		return nil, err
	}
	return refs, nil
}

func collectInlineSQLReferencesFromAny(input any, refs *[]paramsource.InlineReference, err *error) bool {
	switch value := input.(type) {
	case string:
		var parsed []paramsource.InlineReference
		parsed, *err = parseInlineSQLReferences(value)
		if *err != nil {
			return false
		}
		*refs = append(*refs, parsed...)
	case []any:
		for _, item := range value {
			if !collectInlineSQLReferencesFromAny(item, refs, err) {
				return false
			}
		}
	case map[string]any:
		for key, item := range value {
			var parsed []paramsource.InlineReference
			parsed, *err = parseInlineSQLReferences(key)
			if *err != nil {
				return false
			}
			*refs = append(*refs, parsed...)
			if !collectInlineSQLReferencesFromAny(item, refs, err) {
				return false
			}
		}
	case map[string]string:
		for key, item := range value {
			var parsed []paramsource.InlineReference
			parsed, *err = parseInlineSQLReferences(key)
			if *err != nil {
				return false
			}
			*refs = append(*refs, parsed...)
			parsed, *err = parseInlineSQLReferences(item)
			if *err != nil {
				return false
			}
			*refs = append(*refs, parsed...)
		}
	}
	return true
}

func parseInlineSQLReferences(input string) ([]paramsource.InlineReference, error) {
	literals := templating.ScanSQLLiterals(input)
	if len(literals) == 0 {
		return nil, nil
	}
	refs := make([]paramsource.InlineReference, 0, len(literals))
	for _, literal := range literals {
		tok, ok := templating.ParseSQLReference(literal)
		if !ok {
			return nil, fmt.Errorf("invalid sql inline reference %q, expected {{sql.<sourceKey>.<column>}} or {{sql.<sourceKey>[filterColumn=filterValue].<column>}}", literal)
		}
		refs = append(refs, paramsource.InlineReference{
			Expression:   tok.SQL.Expression,
			SourceKey:    tok.SQL.SourceKey,
			FilterColumn: tok.SQL.FilterColumn,
			FilterValue:  tok.SQL.FilterValue,
			Column:       tok.SQL.Column,
		})
	}
	return refs, nil
}
