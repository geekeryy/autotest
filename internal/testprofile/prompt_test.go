package testprofile

import (
	"strings"
	"testing"
)

func TestFormatProfileForPrompt_FullProfile(t *testing.T) {
	profile := &Profile{
		ResponseConvention: &ResponseConvention{
			WrapperFields: []string{"code", "data", "msg"},
			SuccessField:  "code",
			SuccessValue:  float64(0),
			DataField:     "data",
			MessageField:  "msg",
			Confidence:    0.9,
		},
		AuthPatterns: []AuthPattern{
			{HeaderName: "Authorization", ValuePattern: "Bearer {{token}}", Frequency: 0.85},
		},
		FieldProfiles: []FieldProfile{
			{Method: "POST", Path: "/api/v1/orders", Location: "body", FieldPath: "status",
				ValueType: "string", IsEnum: true, ObservedValues: []any{"PENDING", "SHIPPED", "DELIVERED"}},
		},
		DependencyGraph: []EndpointDependency{
			{SourceMethod: "POST", SourcePath: "/api/v1/users", SourceField: "response.body.id",
				TargetMethod: "GET", TargetPath: "/api/v1/users/{id}", TargetField: "path.id", Confidence: 0.9},
		},
	}

	result := FormatProfileForPrompt(profile)

	if !strings.Contains(result, "项目响应约定") {
		t.Error("expected response convention section")
	}
	if !strings.Contains(result, "code") {
		t.Error("expected code field mention")
	}
	if !strings.Contains(result, "认证模式") {
		t.Error("expected auth patterns section")
	}
	if !strings.Contains(result, "Bearer") {
		t.Error("expected Bearer pattern")
	}
	if !strings.Contains(result, "字段枚举值") {
		t.Error("expected field enum section")
	}
	if !strings.Contains(result, "PENDING") {
		t.Error("expected PENDING enum value")
	}
	if !strings.Contains(result, "接口依赖关系") {
		t.Error("expected dependency graph section")
	}
	if !strings.Contains(result, "POST /api/v1/users") {
		t.Error("expected dependency source")
	}
}

func TestFormatProfileForPrompt_NilProfile(t *testing.T) {
	result := FormatProfileForPrompt(nil)
	if result != "" {
		t.Errorf("expected empty string for nil profile, got %q", result)
	}
}

func TestFormatProfileForPrompt_LowConfidence(t *testing.T) {
	profile := &Profile{
		ResponseConvention: &ResponseConvention{
			Confidence: 0.3,
		},
	}
	result := FormatProfileForPrompt(profile)
	if strings.Contains(result, "项目响应约定") {
		t.Error("should not include low-confidence convention")
	}
}
