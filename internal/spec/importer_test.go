package spec

import (
	"context"
	"encoding/json"
	"os"
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
	if endpoint.Fingerprint == "" {
		t.Fatal("expected non-empty fingerprint")
	}
}

func TestEndpointFingerprintIdempotentAndDetectsChanges(t *testing.T) {
	t.Parallel()

	baseDoc := []byte(`
openapi: 3.0.3
info:
  title: Fingerprint API
  version: 1.0.0
paths:
  /items:
    post:
      operationId: createItem
      summary: Create item
      tags: [items, catalog]
      parameters:
        - in: query
          name: dryRun
          schema:
            type: boolean
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

	importer := NewImporter()
	first, err := importer.Import(context.Background(), baseDoc)
	if err != nil {
		t.Fatalf("import base: %v", err)
	}
	if len(first.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(first.Endpoints))
	}
	base := first.Endpoints[0]
	if base.Fingerprint == "" {
		t.Fatal("expected fingerprint on import")
	}

	second, err := importer.Import(context.Background(), baseDoc)
	if err != nil {
		t.Fatalf("re-import base: %v", err)
	}
	if second.Endpoints[0].Fingerprint != base.Fingerprint {
		t.Fatalf("expected stable fingerprint on identical spec, got %q vs %q", second.Endpoints[0].Fingerprint, base.Fingerprint)
	}

	summaryDoc := []byte(`
openapi: 3.0.3
info:
  title: Fingerprint API
  version: 1.0.0
paths:
  /items:
    post:
      operationId: createItem
      summary: Create item (revised)
      tags: [items, catalog]
      parameters:
        - in: query
          name: dryRun
          schema:
            type: boolean
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
	changedSummary, err := importer.Import(context.Background(), summaryDoc)
	if err != nil {
		t.Fatalf("import summary change: %v", err)
	}
	if changedSummary.Endpoints[0].Fingerprint == base.Fingerprint {
		t.Fatal("expected fingerprint to change when summary changes")
	}

	paramsDoc := []byte(`
openapi: 3.0.3
info:
  title: Fingerprint API
  version: 1.0.0
paths:
  /items:
    post:
      operationId: createItem
      summary: Create item
      tags: [items, catalog]
      parameters:
        - in: query
          name: dryRun
          required: true
          schema:
            type: boolean
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
	changedParams, err := importer.Import(context.Background(), paramsDoc)
	if err != nil {
		t.Fatalf("import parameter change: %v", err)
	}
	if changedParams.Endpoints[0].Fingerprint == base.Fingerprint {
		t.Fatal("expected fingerprint to change when parameters change")
	}

	opIDDoc := []byte(`
openapi: 3.0.3
info:
  title: Fingerprint API
  version: 1.0.0
paths:
  /items:
    post:
      operationId: createItemV2
      summary: Create item
      tags: [items, catalog]
      parameters:
        - in: query
          name: dryRun
          schema:
            type: boolean
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
	changedOpID, err := importer.Import(context.Background(), opIDDoc)
	if err != nil {
		t.Fatalf("import operationId change: %v", err)
	}
	if changedOpID.Endpoints[0].Fingerprint == base.Fingerprint {
		t.Fatal("expected fingerprint to change when operationId changes")
	}

	tagsDoc := []byte(`
openapi: 3.0.3
info:
  title: Fingerprint API
  version: 1.0.0
paths:
  /items:
    post:
      operationId: createItem
      summary: Create item
      tags: [catalog]
      parameters:
        - in: query
          name: dryRun
          schema:
            type: boolean
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
	changedTags, err := importer.Import(context.Background(), tagsDoc)
	if err != nil {
		t.Fatalf("import tags change: %v", err)
	}
	if changedTags.Endpoints[0].Fingerprint == base.Fingerprint {
		t.Fatal("expected fingerprint to change when tags change")
	}
}

func TestImporterAllowsSchemaExtraSiblingFields(t *testing.T) {
	t.Parallel()

	doc := []byte(`
openapi: 3.0.3
info:
  title: Extra Siblings API
  version: 1.0.0
components:
  schemas:
    order_detail:
      type: object
      properties:
        TDYS:
          type: string
          enum: ["06 不动产租赁"]
          examples:
            - "06"
          default: "06"
      description: order detail
      BDCZLXXLIST:
        type: array
        items:
          type: string
paths:
  /orders:
    post:
      operationId: createOrder
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/order_detail"
      responses:
        "200":
          description: ok
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import openapi with extra schema siblings: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}
}

func TestImportTMSInvoiceOpenAPI(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../tms-invoice-openapi-docs-1015.yaml")
	if err != nil {
		t.Skip("tms fixture not present:", err)
	}

	result, err := NewImporter().Import(context.Background(), data)
	if err != nil {
		t.Fatalf("import tms openapi: %v", err)
	}
	if len(result.Endpoints) == 0 {
		t.Fatal("expected endpoints from tms openapi")
	}
}

func TestImporterSkipsSchemaDefaultsValidation(t *testing.T) {
	t.Parallel()

	doc := []byte(`
openapi: 3.0.3
info:
  title: Defaults API
  version: 1.0.0
components:
  schemas:
    common_order_req:
      type: object
      properties:
        TDYS:
          type: string
          enum:
            - "01 成品油"
            - "06 不动产租赁"
          default: "06"
paths:
  /orders:
    post:
      operationId: createOrder
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/common_order_req"
      responses:
        "200":
          description: ok
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import openapi with mismatched default/enum: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}
}

func TestImporterParsesOpenAPI31Endpoints(t *testing.T) {
	t.Parallel()

	doc := []byte(`
openapi: 3.1.0
info:
  title: OAS31 API
  version: 1.0.0
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - in: path
          name: id
          required: true
          schema:
            type: string
      responses:
        "200":
          description: ok
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
		t.Fatalf("import openapi 3.1: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}
}

func TestImporterMergesPathLevelParameters(t *testing.T) {
	t.Parallel()

	doc := []byte(`
openapi: 3.0.3
info:
  title: Path Params API
  version: 1.0.0
paths:
  /items/{id}:
    parameters:
      - in: path
        name: id
        required: true
        schema:
          type: string
      - in: query
        name: tenant
        schema:
          type: string
    get:
      operationId: getItem
      parameters:
        - in: query
          name: tenant
          required: true
          schema:
            type: string
      responses:
        "200":
          description: ok
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import path-level parameters: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}

	var requestSchema map[string]any
	if err := json.Unmarshal(result.Endpoints[0].RequestSchema, &requestSchema); err != nil {
		t.Fatalf("decode request schema: %v", err)
	}
	parameters, _ := requestSchema["parameters"].([]any)
	if len(parameters) != 2 {
		t.Fatalf("expected merged path and query parameters, got %#v", parameters)
	}

	byName := map[string]map[string]any{}
	for _, item := range parameters {
		param, _ := item.(map[string]any)
		name, _ := param["name"].(string)
		byName[name] = param
	}
	if byName["id"]["in"] != "path" || byName["id"]["required"] != true {
		t.Fatalf("expected path parameter id, got %#v", byName["id"])
	}
	if byName["tenant"]["required"] != true {
		t.Fatalf("expected operation-level tenant override to win, got %#v", byName["tenant"])
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

func TestImporterEndpointWithoutTagsUsesEmptySlice(t *testing.T) {
	t.Parallel()

	doc := []byte(`
openapi: 3.0.3
info:
  title: No Tags API
  version: 1.0.0
paths:
  /tms/invoice/v4/AllocateInvoices:
    post:
      operationId: allocateInvoices
      summary: 发票开具接口
      responses:
        "200":
          description: success
`)

	result, err := NewImporter().Import(context.Background(), doc)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(result.Endpoints))
	}
	endpoint := result.Endpoints[0]
	if endpoint.Tags == nil {
		t.Fatal("expected non-nil tags slice for operation without OpenAPI tags")
	}
	if len(endpoint.Tags) != 0 {
		t.Fatalf("expected empty tags, got %#v", endpoint.Tags)
	}
}
