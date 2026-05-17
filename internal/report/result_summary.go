package report

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

const DefaultSummaryMaxBodyChars = 8000

// BuildResultSummaryEntry turns a run result into a JSON-friendly map for APIs and AI context.
func BuildResultSummaryEntry(result Result, stepIndex map[uuid.UUID]StepMeta, maxBody int) map[string]any {
	if maxBody <= 0 {
		maxBody = DefaultSummaryMaxBodyChars
	}
	out := map[string]any{
		"id":             result.ID,
		"testCaseId":     result.TestCaseID,
		"status":         string(result.Status),
		"durationMillis": result.DurationMillis,
	}
	if result.StepID != nil {
		out["stepId"] = *result.StepID
		if info, ok := stepIndex[*result.StepID]; ok {
			out["step"] = map[string]any{
				"stepSeq":    info.StepSeq,
				"stepOrder":  info.StepOrder,
				"name":       info.Name,
				"stepType":   info.StepType,
				"testCaseId": info.TestCaseID,
			}
		}
	}
	if result.Error != "" {
		out["error"] = result.Error
	}
	if reqSnap := TruncateJSONSnapshot(result.RequestSnapshot, maxBody); reqSnap != nil {
		out["requestSnapshot"] = reqSnap
	}
	if respSnap := TruncateJSONSnapshot(result.ResponseSnapshot, maxBody); respSnap != nil {
		out["responseSnapshot"] = respSnap
	}
	if assertions := DecodeJSONAny(result.Assertions); assertions != nil {
		out["assertions"] = assertions
	}
	return out
}

// TruncateJSONSnapshot decodes JSON and truncates large string bodies in snapshots.
func TruncateJSONSnapshot(raw json.RawMessage, maxBody int) any {
	value := DecodeJSONAny(raw)
	if value == nil {
		return nil
	}
	asMap, ok := value.(map[string]any)
	if !ok {
		return value
	}
	if body, ok := asMap["body"]; ok {
		if asString, ok := body.(string); ok && len(asString) > maxBody {
			asMap["body"] = asString[:maxBody] + fmt.Sprintf("…<truncated %d chars>", len(asString)-maxBody)
		}
	}
	return asMap
}

// ExtractAssertionFailures flattens failed assertions from a run result for AI context.
func ExtractAssertionFailures(result Result) []map[string]any {
	value := DecodeJSONAny(result.Assertions)
	asSlice, ok := value.([]any)
	if !ok {
		return nil
	}
	out := []map[string]any{}
	for _, entry := range asSlice {
		asMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if passed, ok := asMap["passed"].(bool); ok && passed {
			continue
		}
		copy := map[string]any{
			"resultId":   result.ID,
			"testCaseId": result.TestCaseID,
		}
		for k, v := range asMap {
			copy[k] = v
		}
		out = append(out, copy)
	}
	return out
}
