package spec

import (
	"context"
	"encoding/json"
	"testing"
)

func TestImporterParsesOpenAPIEndpoints(t *testing.T) {
	t.Parallel()

	doc := []byte(`
openapi: 3.0.3
info:
  title: Demo API
  version: 1.0.0
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
security:
  - BearerAuth: []
paths:
  /users:
    post:
      operationId: createUser
      summary: Create user
      tags: [users]
      parameters:
        - in: query
          name: trace
          description: 调试追踪号
          schema:
            type: string
      requestBody:
        required: true
        description: 创建用户请求
        content:
          application/json:
            schema:
              title: 创建用户参数
              type: object
              required: [name]
              properties:
                name:
                  type: string
                  description: 用户姓名
                  minLength: 2
                  maxLength: 20
                  pattern: "^[A-Za-z]+$"
                age:
                  type: integer
                  minimum: 1
                  maximum: 120
                  exclusiveMaximum: true
                  multipleOf: 1
                role:
                  type: string
                  enum: [admin, tester]
                  nullable: true
                tags:
                  type: array
                  minItems: 1
                  maxItems: 3
                  uniqueItems: true
                  items:
                    type: string
      responses:
        "201":
          description: created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                    description: 用户 ID
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import openapi: %v", err)
	}
	if result.Hash == "" {
		t.Fatalf("expected content hash")
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}

	endpoint := result.Endpoints[0]
	if endpoint.Method != "POST" || endpoint.Path != "/users" {
		t.Fatalf("unexpected endpoint: %#v", endpoint)
	}
	if endpoint.OperationID != "createUser" {
		t.Fatalf("unexpected operation id: %s", endpoint.OperationID)
	}
	if endpoint.Summary != "Create user" {
		t.Fatalf("unexpected summary: %s", endpoint.Summary)
	}
	if len(endpoint.RequestSchema) == 0 || len(endpoint.ResponseSchema) == 0 {
		t.Fatalf("expected request and response schemas")
	}
	var requestSchema map[string]any
	if err := json.Unmarshal(endpoint.RequestSchema, &requestSchema); err != nil {
		t.Fatalf("decode request schema: %v", err)
	}
	security, _ := requestSchema["security"].([]any)
	if len(security) != 1 {
		t.Fatalf("expected inherited security requirement, got %#v", requestSchema["security"])
	}
	parameters, _ := requestSchema["parameters"].([]any)
	param, _ := parameters[0].(map[string]any)
	if param["description"] != "调试追踪号" {
		t.Fatalf("expected parameter description, got %#v", param)
	}
	body, _ := requestSchema["body"].(map[string]any)
	properties, _ := body["properties"].(map[string]any)
	name, _ := properties["name"].(map[string]any)
	if body["title"] != "创建用户参数" || body["description"] != "创建用户请求" || name["description"] != "用户姓名" {
		t.Fatalf("expected body field descriptions, got %#v", body)
	}
	if name["minLength"] != float64(2) || name["maxLength"] != float64(20) || name["pattern"] != "^[A-Za-z]+$" {
		t.Fatalf("expected string validation constraints, got %#v", name)
	}
	age, _ := properties["age"].(map[string]any)
	if age["minimum"] != float64(1) || age["maximum"] != float64(120) || age["exclusiveMaximum"] != true || age["multipleOf"] != float64(1) {
		t.Fatalf("expected numeric validation constraints, got %#v", age)
	}
	role, _ := properties["role"].(map[string]any)
	enumValues, _ := role["enum"].([]any)
	if len(enumValues) != 2 || role["nullable"] != true {
		t.Fatalf("expected enum and nullable constraints, got %#v", role)
	}
	tags, _ := properties["tags"].(map[string]any)
	if tags["minItems"] != float64(1) || tags["maxItems"] != float64(3) || tags["uniqueItems"] != true {
		t.Fatalf("expected array validation constraints, got %#v", tags)
	}
	var responseSchema map[string]any
	if err := json.Unmarshal(endpoint.ResponseSchema, &responseSchema); err != nil {
		t.Fatalf("decode response schema: %v", err)
	}
	responseBody, _ := responseSchema["body"].(map[string]any)
	responseProperties, _ := responseBody["properties"].(map[string]any)
	id, _ := responseProperties["id"].(map[string]any)
	if id["description"] != "用户 ID" {
		t.Fatalf("expected response field description, got %#v", responseBody)
	}
}

func TestImporterParsesSwagger2Endpoints(t *testing.T) {
	t.Parallel()

	doc := []byte(`
swagger: "2.0"
info:
  title: Legacy API
  version: 1.0.0
host: example.test
basePath: /api
schemes: [https]
paths:
  /pets:
    get:
      operationId: listPets
      summary: List pets
      produces: [application/json]
      responses:
        "200":
          description: ok
          schema:
            type: array
            items:
              type: object
              properties:
                id:
                  type: integer
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import swagger 2.0: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}
	endpoint := result.Endpoints[0]
	if endpoint.Method != "GET" || endpoint.Path != "/pets" {
		t.Fatalf("unexpected endpoint: %#v", endpoint)
	}
	if endpoint.OperationID != "listPets" {
		t.Fatalf("unexpected operation id: %s", endpoint.OperationID)
	}
	if endpoint.Summary != "List pets" {
		t.Fatalf("unexpected summary: %s", endpoint.Summary)
	}
}

func TestImporterDereferencesSwagger2BodySchemas(t *testing.T) {
	t.Parallel()

	doc := []byte(`
swagger: "2.0"
info:
  title: Login API
  version: 1.0.0
paths:
  /auth/login:
    post:
      operationId: login
      parameters:
        - in: body
          name: credentials
          required: true
          schema:
            $ref: "#/definitions/LoginRequest"
      responses:
        "200":
          description: ok
definitions:
  LoginRequest:
    type: object
    required: [username, password]
    properties:
      username:
        type: string
        example: admin
      password:
        type: string
        example: admin123
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import swagger 2.0: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}

	var request map[string]any
	if err := json.Unmarshal(result.Endpoints[0].RequestSchema, &request); err != nil {
		t.Fatalf("decode request schema: %v", err)
	}
	body, _ := request["body"].(map[string]any)
	properties, _ := body["properties"].(map[string]any)
	username, _ := properties["username"].(map[string]any)
	password, _ := properties["password"].(map[string]any)
	if username["example"] != "admin" || password["example"] != "admin123" {
		t.Fatalf("expected dereferenced login examples, got %#v", body)
	}
}
