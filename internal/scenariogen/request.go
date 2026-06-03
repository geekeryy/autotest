package scenariogen

import (
	"encoding/json"
	"fmt"

	"autotest/internal/scenariogen/depgraph"
	"autotest/internal/spec"
)

func jsonRequestOverride(parts map[string]any) json.RawMessage {
	if len(parts) == 0 {
		return nil
	}
	raw, _ := json.Marshal(parts)
	return raw
}

func bearerAuthOverride() json.RawMessage {
	return jsonRequestOverride(map[string]any{
		"headers": map[string]string{
			"Authorization": "Bearer {{authToken}}",
		},
	})
}

func loginStepConfig(ep spec.Endpoint) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"extracts": []map[string]string{{
			"name": "authToken",
			"from": depgraph.TokenResponsePath(ep),
		}},
	})
	return raw
}

func pathIDOverride(sourceStepSeq int, producerPath string) json.RawMessage {
	return jsonRequestOverride(map[string]any{
		"variables": map[string]string{
			"id": fmt.Sprintf("{{$steps[%d].%s}}", sourceStepSeq, producerPath),
		},
	})
}

func bodyFieldOverride(sourceStepSeq int, producerPath, field string) json.RawMessage {
	return jsonRequestOverride(map[string]any{
		"body": map[string]any{
			field: fmt.Sprintf("{{$steps[%d].%s}}", sourceStepSeq, producerPath),
		},
	})
}

func defaultAssertions() json.RawMessage {
	return json.RawMessage(`[{"type":"status_code","expected":200}]`)
}

func mergeOverrides(parts ...json.RawMessage) json.RawMessage {
	headers := map[string]string{}
	variables := map[string]string{}
	var body any
	for _, raw := range parts {
		if len(raw) == 0 {
			continue
		}
		var chunk map[string]any
		if json.Unmarshal(raw, &chunk) != nil {
			continue
		}
		if h, ok := chunk["headers"].(map[string]any); ok {
			for k, v := range h {
				headers[k] = fmt.Sprint(v)
			}
		}
		if v, ok := chunk["variables"].(map[string]any); ok {
			for k, val := range v {
				variables[k] = fmt.Sprint(val)
			}
		}
		if b, ok := chunk["body"]; ok {
			body = b
		}
	}
	out := map[string]any{}
	if len(headers) > 0 {
		out["headers"] = headers
	}
	if len(variables) > 0 {
		out["variables"] = variables
	}
	if body != nil {
		out["body"] = body
	}
	if len(out) == 0 {
		return nil
	}
	raw, _ := json.Marshal(out)
	return raw
}

func statusAssertionConfig(expectStatus int) json.RawMessage {
	if expectStatus == 0 || expectStatus == 200 {
		return nil
	}
	raw, _ := json.Marshal(map[string]any{
		"assertions": []map[string]any{{"type": "status_code", "expected": expectStatus}},
	})
	return raw
}

func mergeStepConfig(parts ...json.RawMessage) json.RawMessage {
	merged := map[string]any{}
	for _, raw := range parts {
		if len(raw) == 0 {
			continue
		}
		var chunk map[string]any
		if json.Unmarshal(raw, &chunk) != nil {
			continue
		}
		for k, v := range chunk {
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	out, _ := json.Marshal(merged)
	return out
}
