// Package sampler generates sample request parameter values from a stored
// OpenAPI requestSchema JSON (as produced by the spec importer).
// It is kept independent of both the case and generator packages to avoid
// import cycles: generator → case and case → sampler are both allowed.
package sampler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

// RequestSample holds generated parameter values for a request.
//
// Headers are populated for callers that need a complete sample (the auto
// generator uses them to attach Content-Type for JSON bodies). Callers that
// only want path/query/body samples (e.g. the manual run "generate params"
// API) can simply ignore the Headers field.
type RequestSample struct {
	Headers  map[string]string `json:"headers"`
	Query    map[string]string `json:"query"`
	Path     map[string]string `json:"path"`
	Body     any               `json:"body"`
	Security any               `json:"security,omitempty"`
}

// FromSchema generates a RequestSample from a stored requestSchema JSON blob.
// It covers query parameters, request headers, path variables and the JSON body.
//
// For each field the precedence is: example → default → enum[0] → semantic
// fallback derived from the field name and JSON schema format. The fallback
// path always returns a freshly randomised value so that calling FromSchema
// repeatedly produces different samples for fields that lack an explicit
// example/default/enum.
func FromSchema(raw json.RawMessage) RequestSample {
	sample := RequestSample{
		Headers: map[string]string{},
		Query:   map[string]string{},
		Path:    map[string]string{},
		Body:    map[string]any{},
	}

	var request map[string]any
	if err := json.Unmarshal(raw, &request); err != nil {
		return sample
	}

	if params, _ := request["parameters"].([]any); len(params) > 0 {
		for _, item := range params {
			param, _ := item.(map[string]any)
			name, _ := param["name"].(string)
			location, _ := param["in"].(string)
			if name == "" {
				continue
			}
			value := toString(valueFromSchema(name, schemaMap(param["schema"])))
			switch location {
			case "query":
				sample.Query[name] = value
			case "header":
				sample.Headers[name] = value
			case "path":
				sample.Path[name] = value
			}
		}
	}

	body, _ := request["body"].(map[string]any)
	if body != nil {
		sample.Body = valueFromSchema("", body)
		if _, ok := sample.Headers["Content-Type"]; !ok {
			sample.Headers["Content-Type"] = "application/json"
		}
	}
	if security, ok := request["security"]; ok {
		sample.Security = security
	}
	return sample
}

func schemaMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

func toString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

// valueFromSchema picks a sample value for the given field. The fieldName is
// used for semantic-aware fallback (e.g. a string field named "email" yields
// a random email address) when the schema does not provide example/default/enum.
func valueFromSchema(fieldName string, schema map[string]any) any {
	if example, ok := schema["example"]; ok {
		return example
	}
	if def, ok := schema["default"]; ok {
		return def
	}
	if enumValues, ok := schema["enum"].([]any); ok && len(enumValues) > 0 {
		return enumValues[0]
	}
	if allOf, ok := schema["allOf"].([]any); ok && len(allOf) > 0 {
		out := map[string]any{}
		for _, item := range allOf {
			itemSchema, _ := item.(map[string]any)
			if sample, ok := valueFromSchema(fieldName, itemSchema).(map[string]any); ok {
				for key, val := range sample {
					out[key] = val
				}
			}
		}
		return out
	}
	if oneOf, ok := schema["oneOf"].([]any); ok && len(oneOf) > 0 {
		return valueFromSchema(fieldName, schemaMap(oneOf[0]))
	}
	if anyOf, ok := schema["anyOf"].([]any); ok && len(anyOf) > 0 {
		return valueFromSchema(fieldName, schemaMap(anyOf[0]))
	}

	schemaType, _ := schema["type"].(string)
	// When the schema omits an explicit type, infer it from example or default
	// so that gofakeit can still produce a correctly typed random value.
	if schemaType == "" {
		if ex, ok := schema["example"]; ok {
			schemaType = inferSchemaType(ex)
		} else if def, ok := schema["default"]; ok {
			schemaType = inferSchemaType(def)
		}
	}

	switch schemaType {
	case "object":
		return objectSample(schema)
	case "array":
		itemSchema, _ := schema["items"].(map[string]any)
		return []any{valueFromSchema(fieldName, itemSchema)}
	case "integer":
		return integerSample(fieldName, schema)
	case "number":
		return numberSample(fieldName, schema)
	case "boolean":
		return gofakeit.Bool()
	case "string":
		return stringSample(fieldName, schema)
	default:
		if props, _ := schema["properties"].(map[string]any); len(props) > 0 {
			return objectSample(schema)
		}
		// Last resort: use the example/default value verbatim when the type
		// cannot be determined any other way.
		if example, ok := schema["example"]; ok {
			return example
		}
		if def, ok := schema["default"]; ok {
			return def
		}
		return map[string]any{}
	}
}

// inferSchemaType returns the JSON Schema type string that corresponds to the
// runtime Go type of v (as produced by json.Unmarshal into an any).
func inferSchemaType(v any) string {
	switch f := v.(type) {
	case string:
		return "string"
	case float64:
		// json.Unmarshal always decodes numbers as float64; treat whole numbers
		// as integers so integerSample is used for fields like "id".
		if f == float64(int64(f)) {
			return "integer"
		}
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	}
	return ""
}

func objectSample(schema map[string]any) map[string]any {
	properties, _ := schema["properties"].(map[string]any)
	out := make(map[string]any, len(properties))
	for name, prop := range properties {
		propSchema, _ := prop.(map[string]any)
		out[name] = valueFromSchema(name, propSchema)
	}
	return out
}

func integerSample(fieldName string, schema map[string]any) int {
	switch normalizeFieldName(fieldName) {
	case "age":
		return gofakeit.IntRange(18, 80)
	case "year":
		return gofakeit.IntRange(2000, time.Now().Year())
	case "month":
		return gofakeit.IntRange(1, 12)
	case "day":
		return gofakeit.IntRange(1, 28)
	case "count", "total", "size", "limit", "page", "pageSize", "pagesize":
		return gofakeit.IntRange(1, 100)
	}
	if format, _ := schema["format"].(string); format == "int64" {
		return gofakeit.IntRange(1, 1_000_000)
	}
	return gofakeit.IntRange(1, 100)
}

func numberSample(fieldName string, schema map[string]any) float64 {
	switch normalizeFieldName(fieldName) {
	case "price", "amount", "total", "cost", "balance":
		return float64(gofakeit.IntRange(1, 10000)) + gofakeit.Float64Range(0, 1)
	case "latitude", "lat":
		return gofakeit.Latitude()
	case "longitude", "lon", "lng":
		return gofakeit.Longitude()
	}
	_ = schema
	return gofakeit.Float64Range(1, 100)
}

// stringSample generates a format-aware and field-name-aware string sample.
// schema.format is checked first; when no format hint is available, the field
// name is inspected to produce a value that matches the field's likely
// semantics. Every call returns a freshly generated value via gofakeit.
func stringSample(fieldName string, schema map[string]any) string {
	format, _ := schema["format"].(string)
	switch format {
	case "date-time":
		return gofakeit.Date().UTC().Format(time.RFC3339)
	case "date":
		return gofakeit.Date().UTC().Format("2006-01-02")
	case "time":
		return gofakeit.Date().UTC().Format("15:04:05")
	case "email":
		return gofakeit.Email()
	case "uuid":
		return gofakeit.UUID()
	case "uri", "url":
		return gofakeit.URL()
	case "hostname":
		return gofakeit.DomainName()
	case "ipv4":
		return gofakeit.IPv4Address()
	case "ipv6":
		return gofakeit.IPv6Address()
	case "password":
		return gofakeit.Password(true, true, true, true, false, 12)
	case "byte":
		return "c3RyaW5n"
	case "binary":
		return "string"
	}

	switch normalizeFieldName(fieldName) {
	case "id", "uuid", "uid", "guid":
		return gofakeit.UUID()
	case "name", "fullname", "displayname", "username", "userName", "user":
		return gofakeit.Name()
	case "firstname":
		return gofakeit.FirstName()
	case "lastname", "surname":
		return gofakeit.LastName()
	case "email", "mail":
		return gofakeit.Email()
	case "phone", "mobile", "tel", "telephone":
		return gofakeit.Phone()
	case "url", "uri", "link", "website":
		return gofakeit.URL()
	case "address":
		return gofakeit.Address().Address
	case "city":
		return gofakeit.City()
	case "country":
		return gofakeit.Country()
	case "zipcode", "postcode", "postalcode":
		return gofakeit.Zip()
	case "color", "colour":
		return gofakeit.Color()
	case "company", "organization", "org":
		return gofakeit.Company()
	case "title":
		return gofakeit.JobTitle()
	case "description", "summary", "content", "remark", "comment":
		return gofakeit.Sentence(8)
	case "role":
		return gofakeit.RandomString([]string{"admin", "tester", "viewer", "developer"})
	case "status":
		return gofakeit.RandomString([]string{"active", "inactive", "pending"})
	case "createdat", "updatedat", "deletedat", "timestamp", "time", "datetime", "date":
		return gofakeit.Date().UTC().Format(time.RFC3339)
	case "ip", "ipaddress":
		return gofakeit.IPv4Address()
	case "token", "accesstoken", "secret", "apikey":
		return gofakeit.LetterN(32)
	case "password", "pwd":
		return gofakeit.Password(true, true, true, true, false, 12)
	}

	return gofakeit.Word()
}

// normalizeFieldName lowercases and strips common separators so that
// semantic matching works for fields like "user_name", "user-name",
// "userName" or "UserName".
func normalizeFieldName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch r {
		case '_', '-', ' ', '.', ':':
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
