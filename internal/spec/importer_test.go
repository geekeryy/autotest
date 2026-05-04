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
      tags: [users]
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
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
