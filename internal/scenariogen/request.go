package scenariogen

import (
	"encoding/json"
	"fmt"
)

func jsonRequestOverride(parts map[string]any) json.RawMessage {
	if len(parts) == 0 {
		return nil
	}
	raw, _ := json.Marshal(parts)
	return raw
}

func loginRequestOverride(username, password string) json.RawMessage {
	return jsonRequestOverride(map[string]any{
		"body": map[string]any{
			"username": username,
			"password": password,
		},
	})
}

func bearerAuthOverride(loginStepSeq int) json.RawMessage {
	return jsonRequestOverride(map[string]any{
		"headers": map[string]string{
			"Authorization": fmt.Sprintf("Bearer {{$steps[%d].response.body.token}}", loginStepSeq),
		},
	})
}

func pathIDOverride(loginOrSourceStepSeq int, bodyPath string) json.RawMessage {
	return jsonRequestOverride(map[string]any{
		"variables": map[string]string{
			"id": fmt.Sprintf("{{$steps[%d].response.%s}}", loginOrSourceStepSeq, bodyPath),
		},
	})
}

func defaultAssertions() json.RawMessage {
	return json.RawMessage(`[{"type":"status_code","expected":200}]`)
}

func statusAssertion(expected int) json.RawMessage {
	raw, _ := json.Marshal([]map[string]any{{"type": "status_code", "expected": expected}})
	return raw
}
