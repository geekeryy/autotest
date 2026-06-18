package builtin

import (
	"encoding/json"
	"strings"
)

func isJSONNull(b json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(b))
	return trimmed == "null"
}
