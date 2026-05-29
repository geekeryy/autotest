package testprofile

import (
	"encoding/json"
	"regexp"
	"strings"
)

var pathParamRegex = regexp.MustCompile(`\{(\w+)\}`)

// InferDependencies analyzes endpoint schemas to infer data flow relationships.
// It uses heuristics based on path parameters, response fields, and CRUD patterns.
//
// Rules:
//  1. A path parameter {xyz} on a GET/PUT/PATCH/DELETE likely comes from
//     a POST on the same resource that returns xyz in its response.
//  2. If POST /resources returns "id" in response, then GET /resources/{id}
//     depends on it.
//  3. Resource names are extracted from URL paths for matching.
type endpointMeta struct {
	info         EndpointInfo
	resource     string
	pathParams   []string
	responseKeys []string
}

func InferDependencies(endpoints []EndpointInfo) []EndpointDependency {
	var deps []EndpointDependency


	metas := make([]endpointMeta, 0, len(endpoints))
	for _, ep := range endpoints {
		params := extractPathParams(ep.Path)
		resource := extractResource(ep.Path)
		responseKeys := extractTopLevelResponseKeys(ep.ResponseSchema)
		metas = append(metas, endpointMeta{
			info:         ep,
			resource:     resource,
			pathParams:   params,
			responseKeys: responseKeys,
		})
	}

	// For each endpoint with path params, find a producer endpoint.
	for _, consumer := range metas {
		if len(consumer.pathParams) == 0 {
			continue
		}
		method := strings.ToUpper(consumer.info.Method)
		if method == "POST" {
			continue
		}

		for _, param := range consumer.pathParams {
			producer := findProducer(consumer, param, metas)
			if producer == nil {
				continue
			}
			deps = append(deps, EndpointDependency{
				SourceMethod: producer.info.Method,
				SourcePath:   producer.info.Path,
				SourceField:  matchedResponseField(param, producer.responseKeys),
				TargetMethod: consumer.info.Method,
				TargetPath:   consumer.info.Path,
				TargetField:  "path." + param,
				Confidence:   computeConfidence(consumer, *producer, param),
			})
		}
	}

	return deps
}

func findProducer(consumer endpointMeta, param string, all []endpointMeta) *endpointMeta {
	// Priority 1: POST on same resource that has matching response key.
	for i := range all {
		ep := &all[i]
		if strings.ToUpper(ep.info.Method) != "POST" {
			continue
		}
		if ep.resource == "" || consumer.resource == "" {
			continue
		}
		if !isSameResource(ep.resource, consumer.resource) {
			continue
		}
		if hasMatchingResponseKey(param, ep.responseKeys) {
			return ep
		}
	}

	// Priority 2: Any POST that returns the matching field.
	for i := range all {
		ep := &all[i]
		if strings.ToUpper(ep.info.Method) != "POST" {
			continue
		}
		if hasMatchingResponseKey(param, ep.responseKeys) {
			return ep
		}
	}

	return nil
}

func extractPathParams(path string) []string {
	matches := pathParamRegex.FindAllStringSubmatch(path, -1)
	params := make([]string, 0, len(matches))
	for _, m := range matches {
		params = append(params, m[1])
	}
	return params
}

// extractResource gets the primary resource name from a URL path.
// /api/v1/users/{id}/orders → "orders"
// /api/v1/users/{id} → "users"
// /api/v1/users → "users"
func extractResource(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	// Walk backwards, skip path params and version segments.
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if strings.HasPrefix(seg, "{") {
			continue
		}
		if isVersionSegment(seg) {
			continue
		}
		if isCommonPrefix(seg) {
			continue
		}
		return strings.ToLower(seg)
	}
	return ""
}

func isVersionSegment(s string) bool {
	return strings.HasPrefix(s, "v") && len(s) <= 4 && len(s) >= 2
}

func isCommonPrefix(s string) bool {
	lower := strings.ToLower(s)
	return lower == "api" || lower == "apis" || lower == "rest"
}

func extractTopLevelResponseKeys(responseSchema json.RawMessage) []string {
	if len(responseSchema) == 0 {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal(responseSchema, &schema); err != nil {
		return nil
	}

	// Try: schema.body.properties (typical OpenAPI parsed format)
	if body, ok := schema["body"].(map[string]any); ok {
		if props, ok := body["properties"].(map[string]any); ok {
			return mapKeys(props)
		}
	}

	// Try: schema.properties directly
	if props, ok := schema["properties"].(map[string]any); ok {
		return mapKeys(props)
	}

	// Try: nested data field
	if body, ok := schema["body"].(map[string]any); ok {
		if dataProps, ok := body["properties"].(map[string]any); ok {
			if dataProp, ok := dataProps["data"].(map[string]any); ok {
				if innerProps, ok := dataProp["properties"].(map[string]any); ok {
					keys := mapKeys(innerProps)
					for i := range keys {
						keys[i] = "data." + keys[i]
					}
					return keys
				}
			}
		}
	}

	return nil
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func hasMatchingResponseKey(param string, responseKeys []string) bool {
	normalizedParam := strings.ToLower(param)
	for _, key := range responseKeys {
		normalizedKey := strings.ToLower(key)
		parts := strings.Split(normalizedKey, ".")
		lastPart := parts[len(parts)-1]
		if lastPart == normalizedParam {
			return true
		}
		if normalizedParam == "id" && (lastPart == "id" || strings.HasSuffix(lastPart, "id")) {
			return true
		}
	}
	return false
}

func matchedResponseField(param string, responseKeys []string) string {
	normalizedParam := strings.ToLower(param)
	for _, key := range responseKeys {
		parts := strings.Split(strings.ToLower(key), ".")
		lastPart := parts[len(parts)-1]
		if lastPart == normalizedParam {
			return "body." + key
		}
		if normalizedParam == "id" && lastPart == "id" {
			return "body." + key
		}
	}
	return "body." + param
}

func isSameResource(a, b string) bool {
	a = strings.ToLower(strings.TrimSuffix(a, "s"))
	b = strings.ToLower(strings.TrimSuffix(b, "s"))
	return a == b
}

func computeConfidence(consumer endpointMeta, producer endpointMeta, param string) float64 {
	confidence := 0.5
	if isSameResource(producer.resource, consumer.resource) {
		confidence += 0.3
	}
	if strings.ToUpper(producer.info.Method) == "POST" {
		confidence += 0.1
	}
	if strings.ToLower(param) == "id" {
		confidence += 0.1
	}
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}
