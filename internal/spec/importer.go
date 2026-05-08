package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oasdiff/yaml"
)

type Importer struct{}

func NewImporter() *Importer {
	return &Importer{}
}

func (i *Importer) Import(ctx context.Context, data []byte) (*ImportResult, error) {
	doc, err := loadDocument(ctx, data)
	if err != nil {
		return nil, err
	}

	endpoints := collectEndpoints(doc)
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})

	snapshot := map[string]any{
		"title":     doc.Info.Title,
		"version":   doc.Info.Version,
		"endpoints": endpoints,
	}
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal normalized snapshot: %w", err)
	}

	return &ImportResult{
		Hash:      HashContent(data),
		Title:     doc.Info.Title,
		Version:   doc.Info.Version,
		Endpoints: endpoints,
		Snapshot:  rawSnapshot,
	}, nil
}

func loadDocument(ctx context.Context, data []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	if isSwagger2(data) {
		return loadSwagger2(ctx, loader, data)
	}

	doc, err := loader.LoadFromData(data)
	if err == nil {
		if err := doc.Validate(ctx); err != nil {
			return nil, fmt.Errorf("validate openapi document: %w", err)
		}
		return doc, nil
	}

	var doc2 openapi2.T
	if yamlErr := yaml.Unmarshal(data, &doc2); yamlErr != nil || doc2.Swagger == "" {
		return nil, fmt.Errorf("load openapi document: %w", err)
	}
	return convertSwagger2(ctx, loader, &doc2)
}

func isSwagger2(data []byte) bool {
	var meta map[string]any
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return false
	}
	swagger, _ := meta["swagger"].(string)
	return swagger != ""
}

func loadSwagger2(ctx context.Context, loader *openapi3.Loader, data []byte) (*openapi3.T, error) {
	var doc2 openapi2.T
	if err := yaml.Unmarshal(data, &doc2); err != nil {
		return nil, fmt.Errorf("load swagger 2.0 document: %w", err)
	}
	return convertSwagger2(ctx, loader, &doc2)
}

func convertSwagger2(ctx context.Context, loader *openapi3.Loader, doc2 *openapi2.T) (*openapi3.T, error) {
	converted, convErr := openapi2conv.ToV3WithLoader(doc2, loader, nil)
	if convErr != nil {
		return nil, fmt.Errorf("convert swagger 2.0 document: %w", convErr)
	}
	if err := converted.Validate(ctx); err != nil {
		return nil, fmt.Errorf("validate converted swagger document: %w", err)
	}
	return converted, nil
}

func collectEndpoints(doc *openapi3.T) []Endpoint {
	if doc.Paths == nil {
		return nil
	}

	var endpoints []Endpoint
	for path, item := range doc.Paths.Map() {
		for _, op := range operations(item) {
			if op.operation == nil {
				continue
			}

			req := requestSchema(op.operation, doc.Security)
			resp := responseSchema(op.operation)
			fingerprint := endpointFingerprint(op.method, path, op.operation.OperationID)
			endpoints = append(endpoints, Endpoint{
				Method:         strings.ToUpper(op.method),
				Path:           path,
				OperationID:    op.operation.OperationID,
				Summary:        op.operation.Summary,
				Tags:           append([]string(nil), op.operation.Tags...),
				RequestSchema:  req,
				ResponseSchema: resp,
				Fingerprint:    fingerprint,
			})
		}
	}
	return endpoints
}

type pathOperation struct {
	method    string
	operation *openapi3.Operation
}

func operations(item *openapi3.PathItem) []pathOperation {
	if item == nil {
		return nil
	}
	return []pathOperation{
		{method: httpMethodGet, operation: item.Get},
		{method: httpMethodPost, operation: item.Post},
		{method: httpMethodPut, operation: item.Put},
		{method: httpMethodPatch, operation: item.Patch},
		{method: httpMethodDelete, operation: item.Delete},
		{method: httpMethodHead, operation: item.Head},
		{method: httpMethodOptions, operation: item.Options},
		{method: httpMethodTrace, operation: item.Trace},
	}
}

const (
	httpMethodGet     = "get"
	httpMethodPost    = "post"
	httpMethodPut     = "put"
	httpMethodPatch   = "patch"
	httpMethodDelete  = "delete"
	httpMethodHead    = "head"
	httpMethodOptions = "options"
	httpMethodTrace   = "trace"
)

func requestSchema(op *openapi3.Operation, globalSecurity openapi3.SecurityRequirements) json.RawMessage {
	payload := map[string]any{
		"parameters": parameters(op.Parameters),
	}
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		if schema := contentSchema(op.RequestBody.Value.Content); len(schema) > 0 {
			if op.RequestBody.Value.Description != "" {
				if _, exists := schema["description"]; !exists {
					schema["description"] = op.RequestBody.Value.Description
				}
			}
			payload["body"] = schema
		}
	}
	if security := securityRequirements(op, globalSecurity); len(security) > 0 {
		payload["security"] = security
	}
	return mustJSON(payload)
}

func securityRequirements(op *openapi3.Operation, globalSecurity openapi3.SecurityRequirements) []map[string][]string {
	var source openapi3.SecurityRequirements
	if op != nil && op.Security != nil {
		source = *op.Security
	} else {
		source = globalSecurity
	}
	if len(source) == 0 {
		return nil
	}

	result := make([]map[string][]string, 0, len(source))
	for _, requirement := range source {
		if len(requirement) == 0 {
			continue
		}
		names := make([]string, 0, len(requirement))
		for name := range requirement {
			names = append(names, name)
		}
		sort.Strings(names)

		item := make(map[string][]string, len(requirement))
		for _, name := range names {
			item[name] = append([]string(nil), requirement[name]...)
		}
		result = append(result, item)
	}
	return result
}

func responseSchema(op *openapi3.Operation) json.RawMessage {
	if op.Responses == nil {
		return []byte(`{}`)
	}

	codes := make([]string, 0, len(op.Responses.Map()))
	for code := range op.Responses.Map() {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		resp := op.Responses.Map()[code]
		if resp == nil || resp.Value == nil {
			continue
		}
		payload := map[string]any{
			"status": code,
			"body":   contentSchema(resp.Value.Content),
		}
		if resp.Value.Description != nil && *resp.Value.Description != "" {
			payload["description"] = *resp.Value.Description
		}
		return mustJSON(payload)
	}

	return []byte(`{}`)
}

func parameters(params openapi3.Parameters) []map[string]any {
	result := make([]map[string]any, 0, len(params))
	for _, param := range params {
		if param == nil || param.Value == nil {
			continue
		}
		item := map[string]any{
			"name":     param.Value.Name,
			"in":       param.Value.In,
			"required": param.Value.Required,
		}
		if param.Value.Description != "" {
			item["description"] = param.Value.Description
		}
		if param.Value.Schema != nil {
			item["schema"] = schemaAsMap(param.Value.Schema)
		}
		result = append(result, item)
	}
	return result
}

func contentSchema(content openapi3.Content) map[string]any {
	if content == nil {
		return nil
	}
	media := content["application/json"]
	if media == nil {
		for _, candidate := range content {
			media = candidate
			break
		}
	}
	if media == nil || media.Schema == nil {
		return nil
	}
	return schemaAsMap(media.Schema)
}

func schemaAsMap(ref *openapi3.SchemaRef) map[string]any {
	return schemaRefAsMap(ref, map[*openapi3.Schema]bool{})
}

func schemaRefAsMap(ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool) map[string]any {
	if ref == nil {
		return nil
	}
	if ref.Value == nil {
		if ref.Ref != "" {
			return map[string]any{"$ref": ref.Ref}
		}
		return nil
	}
	return schemaValueAsMap(ref.Value, seen)
}

func schemaValueAsMap(schema *openapi3.Schema, seen map[*openapi3.Schema]bool) map[string]any {
	if schema == nil {
		return nil
	}
	if seen[schema] {
		return map[string]any{}
	}
	seen[schema] = true
	defer delete(seen, schema)

	out := map[string]any{}
	if schema.Title != "" {
		out["title"] = schema.Title
	}
	if schema.Description != "" {
		out["description"] = schema.Description
	}
	if schema.Type != nil {
		types := schema.Type.Slice()
		if len(types) == 1 {
			out["type"] = types[0]
		} else if len(types) > 1 {
			out["type"] = types
		}
	}
	if schema.Format != "" {
		out["format"] = schema.Format
	}
	if schema.Example != nil {
		out["example"] = schema.Example
	}
	if schema.Default != nil {
		out["default"] = schema.Default
	}
	if len(schema.Enum) > 0 {
		out["enum"] = schema.Enum
	}
	if len(schema.Required) > 0 {
		out["required"] = append([]string(nil), schema.Required...)
	}
	if len(schema.Properties) > 0 {
		properties := make(map[string]any, len(schema.Properties))
		for name, property := range schema.Properties {
			properties[name] = schemaRefAsMap(property, seen)
		}
		out["properties"] = properties
	}
	if schema.Items != nil {
		out["items"] = schemaRefAsMap(schema.Items, seen)
	}
	if len(schema.AllOf) > 0 {
		out["allOf"] = schemaRefsAsSlice(schema.AllOf, seen)
	}
	if len(schema.OneOf) > 0 {
		out["oneOf"] = schemaRefsAsSlice(schema.OneOf, seen)
	}
	if len(schema.AnyOf) > 0 {
		out["anyOf"] = schemaRefsAsSlice(schema.AnyOf, seen)
	}
	if schema.AdditionalProperties.Schema != nil {
		out["additionalProperties"] = schemaRefAsMap(schema.AdditionalProperties.Schema, seen)
	} else if schema.AdditionalProperties.Has != nil {
		out["additionalProperties"] = *schema.AdditionalProperties.Has
	}
	return out
}

func schemaRefsAsSlice(refs openapi3.SchemaRefs, seen map[*openapi3.Schema]bool) []any {
	out := make([]any, 0, len(refs))
	for _, ref := range refs {
		out = append(out, schemaRefAsMap(ref, seen))
	}
	return out
}

func mustJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func endpointFingerprint(method, path, operationID string) string {
	return HashContent([]byte(strings.ToUpper(method) + "\x00" + path + "\x00" + operationID))
}
