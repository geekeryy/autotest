package aiskill

import (
	"encoding/json"
	"testing"
)

func TestPresetSkills_HasSmartScenarioGen(t *testing.T) {
	skills := PresetSkills()
	var found *PresetSkill
	for i := range skills {
		if skills[i].Name == "smart_scenario_gen" {
			found = &skills[i]
			break
		}
	}
	if found == nil {
		t.Fatal("smart_scenario_gen skill not found in PresetSkills()")
	}
	if found.Label != "智能场景生成" {
		t.Fatalf("unexpected label: %s", found.Label)
	}
	if len(found.ToolDomains) == 0 {
		t.Fatal("smart_scenario_gen has no tool domains")
	}
	// Must include scenarios and paramsource
	has := func(d string) bool {
		for _, td := range found.ToolDomains {
			if td == d {
				return true
			}
		}
		return false
	}
	if !has("scenarios") {
		t.Error("smart_scenario_gen missing 'scenarios' domain")
	}
	if !has("paramsource") {
		t.Error("smart_scenario_gen missing 'paramsource' domain")
	}
	if !has("meta") {
		t.Error("smart_scenario_gen missing 'meta' domain")
	}
}

func TestSmartScenarioGen_MatchesKeywords(t *testing.T) {
	skills := PresetSkills()
	reg := NewRegistry(skillsToPtr(skills))

	tests := []struct {
		input   string
		want    bool
		desc    string
	}{
		{"帮我生成智能场景", true, "keyword '智能场景'"},
		{"请语义分析一下接口", true, "keyword '语义分析'"},
		{"帮我分析隐式依赖", true, "intent '隐式依赖'"},
		{"生成业务流程场景", true, "intent '业务流程场景'"},
		{"智能生成测试用例", true, "intent '智能生成'"},
		{"帮我创建一个场景", false, "no trigger match"},
		{"查看接口列表", false, "unrelated query"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			matched := reg.Match(tt.input, nil)
			found := false
			for _, sk := range matched {
				if sk.Name == "smart_scenario_gen" {
					found = true
					break
				}
			}
			if found != tt.want {
				t.Errorf("input=%q, want match=%v, got match=%v", tt.input, tt.want, found)
			}
		})
	}
}

func TestSmartScenarioGen_MatchesPageContext(t *testing.T) {
	skills := PresetSkills()
	reg := NewRegistry(skillsToPtr(skills))

	// smart_scenario_gen has no page triggers, so page context alone should not match
	page := json.RawMessage(`{"path":"/scenarios","serviceId":"abc"}`)
	matched := reg.Match("", page)
	for _, sk := range matched {
		if sk.Name == "smart_scenario_gen" {
			t.Error("smart_scenario_gen should not match on page context alone")
		}
	}
}

func skillsToPtr(skills []PresetSkill) []*Skill {
	out := make([]*Skill, len(skills))
	for i := range skills {
		out[i] = &Skill{
			Name:           skills[i].Name,
			Label:          skills[i].Label,
			Description:    skills[i].Description,
			PromptTemplate: skills[i].PromptTemplate,
			ToolDomains:    skills[i].ToolDomains,
			Triggers:       skills[i].Triggers,
			Enabled:        true,
		}
	}
	return out
}
