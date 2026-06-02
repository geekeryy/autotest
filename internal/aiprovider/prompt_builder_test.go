package aiprovider

import (
	"strings"
	"testing"
)

func TestPromptBuilder_SmartScenarioLayerIncludedWhenSkillActive(t *testing.T) {
	pb := NewAssistantPromptBuilder()

	// When smart_scenario_gen skill is active, the layer should be included
	prompt := pb.Build(PromptContext{
		Action:     "assistant_chat",
		HasSkill:   true,
		SkillName:  "smart_scenario_gen",
	})
	if !strings.Contains(prompt, "智能场景生成工作流") {
		t.Error("expected smartScenarioWorkflowPrompt in output when skill is active")
	}
	if !strings.Contains(prompt, "语义角色分类") {
		t.Error("expected semantic analysis instructions when skill is active")
	}
	if !strings.Contains(prompt, "隐式依赖") {
		t.Error("expected implicit dependency instructions when skill is active")
	}
}

func TestPromptBuilder_SmartScenarioLayerExcludedWhenSkillInactive(t *testing.T) {
	pb := NewAssistantPromptBuilder()

	// When no skill is active, the layer should NOT be included
	prompt := pb.Build(PromptContext{
		Action: "assistant_chat",
	})
	if strings.Contains(prompt, "智能场景生成工作流") {
		t.Error("smartScenarioWorkflowPrompt should NOT be in output when no skill is active")
	}
}

func TestPromptBuilder_SmartScenarioLayerExcludedForOtherSkill(t *testing.T) {
	pb := NewAssistantPromptBuilder()

	// When a different skill is active, the layer should NOT be included
	prompt := pb.Build(PromptContext{
		Action:    "assistant_chat",
		HasSkill:  true,
		SkillName: "failure_diagnosis",
	})
	if strings.Contains(prompt, "智能场景生成工作流") {
		t.Error("smartScenarioWorkflowPrompt should NOT be in output for other skills")
	}
}

func TestPromptBuilder_ScenarioWorkflowAlwaysIncluded(t *testing.T) {
	pb := NewAssistantPromptBuilder()

	// The base scenarioWorkflowPrompt (priority 20) should always be present
	prompt := pb.Build(PromptContext{Action: "assistant_chat"})
	if !strings.Contains(prompt, "场景生成工作流") {
		t.Error("expected base scenarioWorkflowPrompt in all assistant prompts")
	}
}
