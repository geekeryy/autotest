// Package sampler generates sample request parameter values from a stored
// OpenAPI requestSchema JSON (as produced by the spec importer).
// It is kept independent of both the case and generator packages to avoid
// import cycles: generator → case and case → sampler are both allowed.
package sampler

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
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

// ProfileLookup provides historically observed values for a request field.
// Implementations wrap testprofile.FieldProfile data so the sampler can prefer
// real-world values over random generation.
type ProfileLookup interface {
	// Lookup returns observed values for the given field path within a request
	// body (dot-separated, e.g. "status" or "address.city"). Returns nil if
	// no historical data is available for the field.
	Lookup(fieldPath string) []any
}

// DependencyHint marks a field as depending on an upstream endpoint's response.
type DependencyHint struct {
	FieldPath    string `json:"fieldPath"`    // consumer field path, e.g. "userId" or "orderId"
	SourceMethod string `json:"sourceMethod"` // producer HTTP method
	SourcePath   string `json:"sourcePath"`   // producer path
	SourceField  string `json:"sourceField"`  // producer response field path, e.g. "response.body.data.id"
	ConsumerKind string `json:"consumerKind"` // "path_param" | "body_field"
}

// DependencyHints groups dependency hints for one endpoint.
type DependencyHints struct {
	Hints []DependencyHint
}

// HasDependency returns true if the given body field path has a dependency hint.
func (dh *DependencyHints) HasDependency(fieldPath string) bool {
	if dh == nil {
		return false
	}
	for _, h := range dh.Hints {
		if h.FieldPath == fieldPath {
			return true
		}
	}
	return false
}

// Lookup returns the dependency hint for a field, or nil.
func (dh *DependencyHints) Lookup(fieldPath string) *DependencyHint {
	if dh == nil {
		return nil
	}
	for i := range dh.Hints {
		if dh.Hints[i].FieldPath == fieldPath {
			return &dh.Hints[i]
		}
	}
	return nil
}

// Options tweaks how FromSchemaWithOptions generates fallback values.
type Options struct {
	// PreferMockTags makes the sampler emit `{{$mock.<helper>}}` placeholder
	// strings (instead of concrete random values) for *string* fields whose
	// format or name maps to a runtime mock helper. Non-string fields and
	// strings without a clear semantic mapping continue to receive concrete
	// random values so the generated body remains a valid JSON shape. The
	// example → default → enum precedence still wins over mock tags.
	//
	// The runner's `mockdata.Expand` evaluates the placeholders on each
	// request, so users no longer need to re-click "生成参数" to refresh
	// dynamic fields like uuid / email / createdAt between runs.
	PreferMockTags bool

	// Profile, when set, provides historically observed values for fields.
	// The sampler uses these values (randomly picking one) before falling
	// back to semantic heuristics. This makes generated parameters match
	// real-world data patterns learned from past runs.
	Profile ProfileLookup

	// Dependencies, when set, marks fields that depend on upstream endpoint
	// responses. The sampler will skip generating random values for these
	// fields, leaving a placeholder that indicates they should be filled
	// via $steps[N] references in scenario context.
	Dependencies *DependencyHints
}

// FromSchema generates a RequestSample with the default options. It is kept
// for callers (e.g. the OpenAPI importer / generator package) that want
// stable concrete values baked into the saved template at creation time.
func FromSchema(raw json.RawMessage) RequestSample {
	return FromSchemaWithOptions(raw, Options{})
}

// FromSchemaWithOptions is the configurable variant. See Options for details.
//
// For each field the precedence is: example → default → enum[0] → semantic
// fallback derived from the field name and JSON schema format. The fallback
// path produces fresh randomised values (or `{{$mock.<helper>}}` placeholders
// when Options.PreferMockTags is set) so callers can opt in to runtime-time
// random generation without losing the stable importer behaviour.
func FromSchemaWithOptions(raw json.RawMessage, opts Options) RequestSample {
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
			// Skip path params that depend on upstream responses.
			if location == "path" && opts.Dependencies != nil && opts.Dependencies.HasDependency(name) {
				hint := opts.Dependencies.Lookup(name)
				sample.Path[name] = fmt.Sprintf("{{__DEPENDENCY:%s %s→%s__}}", hint.SourceMethod, hint.SourcePath, hint.SourceField)
				continue
			}
			value := toString(valueFromSchema(name, schemaMap(param["schema"]), opts))
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
		sample.Body = valueFromSchema("", body, opts)
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
func valueFromSchema(fieldName string, schema map[string]any, opts Options) any {
	return valueFromSchemaWithPath(fieldName, schema, opts, fieldName)
}

// valueFromSchemaWithPath is the internal variant that tracks the full dot-separated
// field path for profile lookups. fieldPath is used to query historical values;
// fieldName is the short name used for semantic matching.
func valueFromSchemaWithPath(fieldName string, schema map[string]any, opts Options, fieldPath string) any {
	if example, ok := schema["example"]; ok {
		return example
	}
	if def, ok := schema["default"]; ok {
		return def
	}
	if enumValues, ok := schema["enum"].([]any); ok && len(enumValues) > 0 {
		return enumValues[0]
	}

	// Query profile for historically observed values before semantic fallback.
	if opts.Profile != nil && fieldPath != "" {
		if observed := opts.Profile.Lookup(fieldPath); len(observed) > 0 {
			return pickRandom(observed)
		}
	}

	if allOf, ok := schema["allOf"].([]any); ok && len(allOf) > 0 {
		out := map[string]any{}
		for _, item := range allOf {
			itemSchema, _ := item.(map[string]any)
			if sample, ok := valueFromSchemaWithPath(fieldName, itemSchema, opts, fieldPath).(map[string]any); ok {
				for key, val := range sample {
					out[key] = val
				}
			}
		}
		return out
	}
	if oneOf, ok := schema["oneOf"].([]any); ok && len(oneOf) > 0 {
		return valueFromSchemaWithPath(fieldName, schemaMap(oneOf[0]), opts, fieldPath)
	}
	if anyOf, ok := schema["anyOf"].([]any); ok && len(anyOf) > 0 {
		return valueFromSchemaWithPath(fieldName, schemaMap(anyOf[0]), opts, fieldPath)
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
		return objectSampleWithPath(schema, opts, fieldPath)
	case "array":
		itemSchema, _ := schema["items"].(map[string]any)
		return []any{valueFromSchemaWithPath(fieldName, itemSchema, opts, fieldPath+"[0]")}
	case "integer":
		return integerSample(fieldName, schema)
	case "number":
		return numberSample(fieldName, schema)
	case "boolean":
		return gofakeit.Bool()
	case "string":
		return stringSample(fieldName, schema, opts)
	default:
		if props, _ := schema["properties"].(map[string]any); len(props) > 0 {
			return objectSampleWithPath(schema, opts, fieldPath)
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

func objectSampleWithPath(schema map[string]any, opts Options, parentPath string) map[string]any {
	properties, _ := schema["properties"].(map[string]any)
	out := make(map[string]any, len(properties))
	for name, prop := range properties {
		childPath := name
		if parentPath != "" {
			childPath = parentPath + "." + name
		}
		// Skip fields that depend on upstream endpoint responses.
		if opts.Dependencies != nil && opts.Dependencies.HasDependency(name) {
			hint := opts.Dependencies.Lookup(name)
			out[name] = fmt.Sprintf("{{__DEPENDENCY:%s %s→%s__}}", hint.SourceMethod, hint.SourcePath, hint.SourceField)
			continue
		}
		propSchema, _ := prop.(map[string]any)
		out[name] = valueFromSchemaWithPath(name, propSchema, opts, childPath)
	}
	return out
}

// pickRandom returns a random element from the slice.
func pickRandom(values []any) any {
	if len(values) == 0 {
		return nil
	}
	return values[rand.N(len(values))]
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
//
// When opts.PreferMockTags is set, fields whose semantics map to a runtime
// mock helper return a `{{$mock.<helper>}}` placeholder string instead, so
// the runner produces a fresh value on every request without forcing the
// user to re-click the "生成参数" button between runs.
func stringSample(fieldName string, schema map[string]any, opts Options) string {
	if opts.PreferMockTags {
		if tag := mockTagForString(fieldName, schema); tag != "" {
			return tag
		}
	}

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

// mockTagForString maps a string field's `format` and (failing that) its
// normalised name to a runtime `{{$mock.<helper>}}` placeholder. It returns
// an empty string when no helper is a good semantic match so the caller
// falls back to a concrete random value. Helper names mirror the canonical
// list documented by `internal/mockdata.ListHelpers`.
//
// Format and field-name patterns intentionally mirror stringSample: the only
// difference is the *result* (placeholder vs. concrete value) so opting into
// PreferMockTags does not silently lose semantic coverage.
func mockTagForString(fieldName string, schema map[string]any) string {
	format, _ := schema["format"].(string)
	switch format {
	case "date-time":
		return "{{$mock.dateTime}}"
	case "date":
		return "{{$mock.date}}"
	case "email":
		return "{{$mock.email}}"
	case "uuid":
		return "{{$mock.uuid}}"
	case "uri", "url":
		return "{{$mock.url}}"
	case "ipv4":
		return "{{$mock.ipv4}}"
	case "ipv6":
		return "{{$mock.ipv6}}"
	}

	switch normalizeFieldName(fieldName) {
	case "id", "uuid", "uid", "guid":
		return "{{$mock.uuid}}"
	case "name", "fullname", "displayname", "username", "user":
		return "{{$mock.name}}"
	case "firstname":
		return "{{$mock.firstName}}"
	case "lastname", "surname":
		return "{{$mock.lastName}}"
	case "email", "mail":
		return "{{$mock.email}}"
	case "phone", "mobile", "tel", "telephone":
		return "{{$mock.phone}}"
	case "url", "uri", "link", "website":
		return "{{$mock.url}}"
	case "address":
		return "{{$mock.address}}"
	case "city":
		return "{{$mock.city}}"
	case "country":
		return "{{$mock.country}}"
	case "color", "colour":
		return "{{$mock.color}}"
	case "company", "organization", "org":
		return "{{$mock.company}}"
	case "createdat", "updatedat", "deletedat", "timestamp", "datetime":
		return "{{$mock.dateTime}}"
	case "ip", "ipaddress":
		return "{{$mock.ipv4}}"
	case "token", "accesstoken", "secret", "apikey":
		return "{{$mock.string(32)}}"
	case "description", "summary", "content", "remark", "comment":
		return "{{$mock.sentence(8)}}"
	case "role":
		return "{{$mock.pick(admin,tester,viewer,developer)}}"
	case "status":
		return "{{$mock.pick(active,inactive,pending)}}"
	}

	return ""
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
