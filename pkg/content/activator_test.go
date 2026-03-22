package content

import (
	"math"
	"strings"
	"testing"
)

// buildTestRegistry builds a SkillRegistry with two synthetic skills for testing.
func buildTestRegistry() *SkillRegistry {
	reg := &SkillRegistry{
		skills: map[string]SkillDefinition{
			"debug": {
				Name:     "debug",
				Category: "engineering",
				Triggers: []string{"debug", "^panic:"},
			},
			"test-gen": {
				Name:     "test-gen",
				Category: "quality",
				Triggers: []string{"write test", `generate.*test`},
			},
		},
	}
	return reg
}

func TestNewSkillActivator(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 5, map[string]int{"engineering": 20})
	if a == nil {
		t.Fatal("expected non-nil activator")
	}
	if a.maxActive != 5 {
		t.Errorf("maxActive = %d, want 5", a.maxActive)
	}
}

func TestMatch_AutoActivateFalse(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, false, 5, nil)
	ctx := ActivationContext{UserQuery: "debug something"}
	results := a.Match(ctx)
	if results != nil {
		t.Errorf("expected nil when autoActivate=false, got %v", results)
	}
}

func TestMatch_KeywordTrigger(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "please debug this function"}
	results := a.Match(ctx)

	found := false
	for _, r := range results {
		if r.Skill.Name == "debug" && r.Source == "keyword" && r.Score == 0.8 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected keyword match for 'debug', got %v", results)
	}
}

func TestMatch_RegexTrigger(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "panic: index out of range"}
	results := a.Match(ctx)

	found := false
	for _, r := range results {
		if r.Skill.Name == "debug" && r.Source == "regex" && r.Score == 0.9 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected regex match for '^panic:', got %v", results)
	}
}

func TestMatch_CaseInsensitiveKeyword(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "DEBUG the server"}
	results := a.Match(ctx)

	found := false
	for _, r := range results {
		if r.Skill.Name == "debug" {
			found = true
		}
	}
	if !found {
		t.Error("expected case-insensitive keyword match for 'DEBUG'")
	}
}

func TestMatch_NoMatch(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "deploy to production"}
	results := a.Match(ctx)
	if len(results) != 0 {
		t.Errorf("expected no matches, got %v", results)
	}
}

func TestMatch_SortedByScore(t *testing.T) {
	reg := buildTestRegistry()
	// Query matches both keyword trigger and regex trigger for different skills.
	// "generate test" matches test-gen via regex (0.9) and via keyword fallback.
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "generate a test"}
	results := a.Match(ctx)

	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Error("results not sorted by score descending")
		}
	}
}

func TestResolve_CategoryWeight(t *testing.T) {
	reg := buildTestRegistry()
	weights := map[string]int{"engineering": 50} // +50%
	a := NewSkillActivator(reg, true, 10, weights)
	ctx := ActivationContext{UserQuery: "debug something"}
	raw := a.Match(ctx)
	resolved := a.Resolve(raw)

	for _, r := range resolved {
		if r.Skill.Category == "engineering" {
			// original score 0.8 * (1 + 50/100) = 1.2
			want := 0.8 * 1.5
			if math.Abs(r.Score-want) > 1e-9 {
				t.Errorf("score = %f, want %f", r.Score, want)
			}
		}
	}
}

func TestResolve_MaxActive(t *testing.T) {
	reg := &SkillRegistry{
		skills: map[string]SkillDefinition{
			"a": {Name: "a", Category: "c", Triggers: []string{"foo"}},
			"b": {Name: "b", Category: "c", Triggers: []string{"foo"}},
			"c": {Name: "c", Category: "c", Triggers: []string{"foo"}},
		},
	}
	a := NewSkillActivator(reg, true, 2, nil)
	ctx := ActivationContext{UserQuery: "foo bar baz"}
	results := a.Resolve(a.Match(ctx))
	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestFormatActivationNotice_Empty(t *testing.T) {
	notice := FormatActivationNotice(nil)
	if notice != "" {
		t.Errorf("expected empty string, got %q", notice)
	}
}

func TestFormatActivationNotice_Single(t *testing.T) {
	results := []ActivationResult{
		{Skill: SkillDefinition{Name: "debug"}, Reason: "keyword: debug"},
	}
	notice := FormatActivationNotice(results)
	if !strings.HasPrefix(notice, "스킬 활성화:") {
		t.Errorf("unexpected notice format: %q", notice)
	}
	if !strings.Contains(notice, "debug (keyword: debug)") {
		t.Errorf("missing expected content in notice: %q", notice)
	}
}

func TestFormatActivationNotice_Multiple(t *testing.T) {
	results := []ActivationResult{
		{Skill: SkillDefinition{Name: "debug"}, Reason: "keyword: debug"},
		{Skill: SkillDefinition{Name: "test-gen"}, Reason: "regex: generate.*test"},
	}
	notice := FormatActivationNotice(results)
	if !strings.Contains(notice, "debug") || !strings.Contains(notice, "test-gen") {
		t.Errorf("missing skill names in notice: %q", notice)
	}
}

func TestIsRegexTrigger(t *testing.T) {
	tests := []struct {
		trigger string
		want    bool
	}{
		{"debug", false},
		{"^panic:", true},
		{"generate.*test", true}, // contains .*
		{"generate(.*)", true},
		{"(foo|bar)", true},
	}
	for _, tc := range tests {
		got := isRegexTrigger(tc.trigger)
		if got != tc.want {
			t.Errorf("isRegexTrigger(%q) = %v, want %v", tc.trigger, got, tc.want)
		}
	}
}

func TestActivateSkills_ReturnsMatchesAndNotice(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "please debug this"}

	resolved, notice := ActivateSkills(a, ctx)

	if len(resolved) == 0 {
		t.Fatal("expected at least one resolved skill")
	}
	if notice == "" {
		t.Error("expected non-empty notice string")
	}
}

func TestActivateSkills_NoMatch(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "deploy to production"}

	resolved, notice := ActivateSkills(a, ctx)

	if len(resolved) != 0 {
		t.Errorf("expected no resolved skills, got %d", len(resolved))
	}
	if notice != "" {
		t.Errorf("expected empty notice, got %q", notice)
	}
}

func TestActivateSkills_AutoActivateFalse(t *testing.T) {
	reg := buildTestRegistry()
	a := NewSkillActivator(reg, false, 10, nil)
	ctx := ActivationContext{UserQuery: "debug something"}

	resolved, notice := ActivateSkills(a, ctx)

	if resolved != nil {
		t.Errorf("expected nil results when autoActivate=false, got %v", resolved)
	}
	if notice != "" {
		t.Errorf("expected empty notice, got %q", notice)
	}
}

func TestMatch_BadRegexSkipped(t *testing.T) {
	reg := &SkillRegistry{
		skills: map[string]SkillDefinition{
			"bad-regex": {
				Name:     "bad-regex",
				Category: "test",
				Triggers: []string{"(unclosed"},
			},
		},
	}
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "(unclosed anything"}
	// Should not panic; bad pattern is skipped gracefully.
	results := a.Match(ctx)
	for _, r := range results {
		if r.Skill.Name == "bad-regex" {
			t.Error("expected bad regex trigger to be skipped")
		}
	}
}
