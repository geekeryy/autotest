package spec

import (
	"encoding/json"
)

// ExtractResponseBodySchema 从 Endpoint 的 ResponseSchema 中提取 body 部分的 JSON Schema。
// ResponseSchema 的格式为 {"status": "200", "body": {...}}，此函数返回 body 部分。
func ExtractResponseBodySchema(responseSchema json.RawMessage) json.RawMessage {
	if len(responseSchema) == 0 {
		return nil
	}
	var wrapper struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(responseSchema, &wrapper); err != nil {
		return nil
	}
	return wrapper.Body
}
