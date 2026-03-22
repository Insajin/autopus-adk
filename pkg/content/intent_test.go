// Package content_test는 인텐트 게이트 패키지의 테스트이다.
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestDefaultRules(t *testing.T) {
	t.Parallel()

	rules := content.DefaultRules()
	assert.NotEmpty(t, rules)

	// 각 규칙은 Pattern, Priority를 가져야 함
	for _, r := range rules {
		assert.NotEmpty(t, r.Pattern)
		assert.Greater(t, r.Priority, 0)
	}
}

func TestDefaultRules_TelemetryCostRule(t *testing.T) {
	t.Parallel()

	rules := content.DefaultRules()

	var found bool
	for _, r := range rules {
		if r.TargetSkill == "telemetry-cost" {
			found = true
			assert.Equal(t, 22, r.Priority)
			assert.NotEmpty(t, r.Pattern)
		}
	}
	assert.True(t, found, "telemetry-cost rule must be present in DefaultRules")
}

func TestDefaultRules_TelemetrySummaryRule(t *testing.T) {
	t.Parallel()

	rules := content.DefaultRules()

	var found bool
	for _, r := range rules {
		if r.TargetSkill == "telemetry-summary" {
			found = true
			assert.Equal(t, 21, r.Priority)
			assert.NotEmpty(t, r.Pattern)
		}
	}
	assert.True(t, found, "telemetry-summary rule must be present in DefaultRules")
}

func TestDefaultRules_TelemetryCompareRule(t *testing.T) {
	t.Parallel()

	rules := content.DefaultRules()

	var found bool
	for _, r := range rules {
		if r.TargetSkill == "telemetry-compare" {
			found = true
			assert.Equal(t, 19, r.Priority)
			assert.NotEmpty(t, r.Pattern)
		}
	}
	assert.True(t, found, "telemetry-compare rule must be present in DefaultRules")
}

func TestGenerateIntentGateInstruction(t *testing.T) {
	t.Parallel()

	rules := []content.IntentRule{
		{Pattern: "plan.*feature", TargetSkill: "planning", Priority: 10},
		{Pattern: "debug.*error", TargetAgent: "debugger", Priority: 20},
		{Pattern: "test.*", TargetSkill: "tdd", Priority: 15},
	}

	result := content.GenerateIntentGateInstruction(rules)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "planning")
	assert.Contains(t, result, "debugger")
	assert.Contains(t, result, "tdd")
}

func TestIntentRule_TargetSkillOrAgent(t *testing.T) {
	t.Parallel()

	// 스킬 또는 에이전트 중 하나만 설정 가능
	skillRule := content.IntentRule{
		Pattern:     "plan",
		TargetSkill: "planning",
		Priority:    10,
	}
	assert.NotEmpty(t, skillRule.TargetSkill)
	assert.Empty(t, skillRule.TargetAgent)

	agentRule := content.IntentRule{
		Pattern:     "debug",
		TargetAgent: "debugger",
		Priority:    20,
	}
	assert.Empty(t, agentRule.TargetSkill)
	assert.NotEmpty(t, agentRule.TargetAgent)
}

func TestGenerateSkillActivationInstruction_Empty(t *testing.T) {
	t.Parallel()

	result := content.GenerateSkillActivationInstruction(nil)
	assert.Empty(t, result)

	result = content.GenerateSkillActivationInstruction([]content.ActivationResult{})
	assert.Empty(t, result)
}

func TestGenerateSkillActivationInstruction_Single(t *testing.T) {
	t.Parallel()

	results := []content.ActivationResult{
		{
			Skill:  content.SkillDefinition{Name: "debugging"},
			Reason: "keyword: debug",
			Source: "keyword",
		},
	}

	out := content.GenerateSkillActivationInstruction(results)
	assert.Contains(t, out, "## Auto-Activated Skills")
	assert.Contains(t, out, "debugging")
	assert.Contains(t, out, "keyword: debug")
	assert.Contains(t, out, "keyword")
}

func TestGenerateSkillActivationInstruction_Multiple(t *testing.T) {
	t.Parallel()

	results := []content.ActivationResult{
		{
			Skill:  content.SkillDefinition{Name: "debugging"},
			Reason: "keyword: debug",
			Source: "keyword",
		},
		{
			Skill:  content.SkillDefinition{Name: "tdd"},
			Reason: "keyword: test",
			Source: "keyword",
		},
	}

	out := content.GenerateSkillActivationInstruction(results)
	assert.Contains(t, out, "## Auto-Activated Skills")
	assert.Contains(t, out, "debugging")
	assert.Contains(t, out, "tdd")
	assert.Contains(t, out, "Loaded skill instructions follow below.")
}
