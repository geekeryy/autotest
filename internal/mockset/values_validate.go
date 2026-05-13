package mockset

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	maxSetJSONDepth = 12
	maxSetJSONNodes = 256
)

func valueDepthAndNodes(v any) (depth, nodes int) {
	nodes = 1
	switch x := v.(type) {
	case nil, bool, string, json.Number, float64:
		return 1, 1
	case []any:
		maxChildD := 0
		for _, el := range x {
			d, n := valueDepthAndNodes(el)
			nodes += n
			if d > maxChildD {
				maxChildD = d
			}
		}
		return 1 + maxChildD, nodes
	case map[string]any:
		maxChildD := 0
		for _, el := range x {
			d, n := valueDepthAndNodes(el)
			nodes += n
			if d > maxChildD {
				maxChildD = d
			}
		}
		return 1 + maxChildD, nodes
	default:
		return 1, 1
	}
}

// validateAndCanonJSON 校验单项 JSON、递归深度与节点数，并规范为紧凑编码。
func validateAndCanonJSON(raw json.RawMessage) (json.RawMessage, error) {
	b := bytes.TrimSpace(raw)
	if len(b) == 0 {
		return nil, ErrEmptyValueEntry
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValueNotValidJSON, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, ErrValueExtraJSON
	}
	depth, nodes := valueDepthAndNodes(v)
	if depth > maxSetJSONDepth {
		return nil, ErrValueDepthExceeded
	}
	if nodes > maxSetJSONNodes {
		return nil, ErrValueTooManyNodes
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValueNotValidJSON, err)
	}
	return out, nil
}

func normalizeValues(in []json.RawMessage) ([]json.RawMessage, error) {
	if len(in) == 0 {
		return nil, ErrEmptyValues
	}
	out := make([]json.RawMessage, 0, len(in))
	for _, raw := range in {
		canon, err := validateAndCanonJSON(raw)
		if err != nil {
			if errors.Is(err, ErrEmptyValueEntry) || errors.Is(err, ErrValueNotValidJSON) ||
				errors.Is(err, ErrValueExtraJSON) || errors.Is(err, ErrValueDepthExceeded) ||
				errors.Is(err, ErrValueTooManyNodes) {
				return nil, err
			}
			return nil, err
		}
		out = append(out, canon)
	}
	return out, nil
}
