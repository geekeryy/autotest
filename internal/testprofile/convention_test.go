package testprofile

import (
	"encoding/json"
	"testing"
)

func TestInferResponseConvention_StandardWrapper(t *testing.T) {
	responses := make([]json.RawMessage, 0, 10)
	for i := 0; i < 10; i++ {
		responses = append(responses, json.RawMessage(`{"code":0,"msg":"success","data":{"id":"abc"}}`))
	}

	conv := InferResponseConvention(responses)
	if conv == nil {
		t.Fatal("expected non-nil convention")
	}
	if conv.SuccessField != "code" {
		t.Errorf("expected SuccessField=code, got %s", conv.SuccessField)
	}
	if conv.SuccessValue != float64(0) {
		t.Errorf("expected SuccessValue=0, got %v", conv.SuccessValue)
	}
	if conv.DataField != "data" {
		t.Errorf("expected DataField=data, got %s", conv.DataField)
	}
	if conv.MessageField != "msg" {
		t.Errorf("expected MessageField=msg, got %s", conv.MessageField)
	}
}

func TestInferResponseConvention_MixedResponses(t *testing.T) {
	responses := []json.RawMessage{
		json.RawMessage(`{"code":0,"message":"ok","data":{"users":[]}}`),
		json.RawMessage(`{"code":0,"message":"ok","data":{"id":1}}`),
		json.RawMessage(`{"code":0,"message":"success","data":null}`),
		json.RawMessage(`{"code":1,"message":"error","data":null}`),
		json.RawMessage(`{"code":0,"message":"ok","data":{"name":"test"}}`),
	}

	conv := InferResponseConvention(responses)
	if conv == nil {
		t.Fatal("expected non-nil convention")
	}
	if conv.SuccessField != "code" {
		t.Errorf("expected SuccessField=code, got %s", conv.SuccessField)
	}
	if conv.DataField != "data" {
		t.Errorf("expected DataField=data, got %s", conv.DataField)
	}
	if conv.MessageField != "message" {
		t.Errorf("expected MessageField=message, got %s", conv.MessageField)
	}
}

func TestInferResponseConvention_TooFewResponses(t *testing.T) {
	responses := []json.RawMessage{
		json.RawMessage(`{"code":0,"data":{}}`),
		json.RawMessage(`{"code":0,"data":{}}`),
	}
	conv := InferResponseConvention(responses)
	if conv != nil {
		t.Errorf("expected nil for too few responses, got %+v", conv)
	}
}

func TestInferResponseConvention_NoCommonWrapper(t *testing.T) {
	responses := []json.RawMessage{
		json.RawMessage(`{"users":[]}`),
		json.RawMessage(`{"id":1,"name":"test"}`),
		json.RawMessage(`{"items":[1,2,3]}`),
		json.RawMessage(`{"result":"ok"}`),
		json.RawMessage(`{"data":[],"total":5}`),
	}
	conv := InferResponseConvention(responses)
	if conv != nil {
		t.Errorf("expected nil for no common wrapper, got %+v", conv)
	}
}

func TestBuildConventionAssertions(t *testing.T) {
	conv := &ResponseConvention{
		SuccessField: "code",
		SuccessValue: float64(0),
		DataField:    "data",
		Confidence:   0.9,
	}

	assertions := BuildConventionAssertions(conv, 200)
	if len(assertions) != 3 {
		t.Fatalf("expected 3 assertions, got %d", len(assertions))
	}
	if assertions[0]["type"] != "status_code" {
		t.Errorf("first assertion should be status_code")
	}
	if assertions[1]["type"] != "jsonpath" || assertions[1]["path"] != "$.code" {
		t.Errorf("second assertion should check $.code, got %v", assertions[1])
	}
	if assertions[2]["type"] != "jsonpath" || assertions[2]["path"] != "$.data" {
		t.Errorf("third assertion should check $.data, got %v", assertions[2])
	}
}

func TestBuildConventionAssertions_NilConvention(t *testing.T) {
	assertions := BuildConventionAssertions(nil, 201)
	if len(assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(assertions))
	}
	if assertions[0]["expected"] != 201 {
		t.Errorf("expected status 201, got %v", assertions[0]["expected"])
	}
}
