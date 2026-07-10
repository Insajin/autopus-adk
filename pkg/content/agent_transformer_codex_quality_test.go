package content_test

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
)

func TestTransformAgentForCodex_RendersQualityAwareProfiles(t *testing.T) {
	t.Parallel()

	balanced := config.QualityConf{
		Default: "balanced",
		Presets: map[string]config.QualityPreset{
			"balanced": {Agents: map[string]string{
				"planner":  "opus",
				"executor": "sonnet",
				"reviewer": "sonnet",
			}},
		},
	}
	ultra := config.QualityConf{Default: "ultra"}

	tests := []struct {
		name         string
		source       content.AgentSource
		quality      config.QualityConf
		wantModel    string
		wantEffort   string
		forbidEffort string
	}{
		{
			name:       "balanced planner",
			source:     codexProfileSource("planner", "opus", "max"),
			quality:    balanced,
			wantModel:  "gpt-5.6-sol",
			wantEffort: "xhigh",
		},
		{
			name:       "balanced executor",
			source:     codexProfileSource("executor", "opus", "medium"),
			quality:    balanced,
			wantModel:  "gpt-5.6-terra",
			wantEffort: "medium",
		},
		{
			name:       "balanced reviewer",
			source:     codexProfileSource("reviewer", "opus", "high"),
			quality:    balanced,
			wantModel:  "gpt-5.6-terra",
			wantEffort: "high",
		},
		{
			name:       "balanced haiku fallback",
			source:     codexProfileSource("synthetic", "haiku", "low"),
			quality:    balanced,
			wantModel:  "gpt-5.6-luna",
			wantEffort: "low",
		},
		{
			name:       "balanced sonnet preserves declared max",
			source:     codexProfileSource("synthetic-max", "sonnet", "max"),
			quality:    balanced,
			wantModel:  "gpt-5.6-terra",
			wantEffort: "max",
		},
		{
			name:         "ultra worker does not delegate",
			source:       codexProfileSource("executor", "sonnet", "medium"),
			quality:      ultra,
			wantModel:    "gpt-5.6-sol",
			wantEffort:   "max",
			forbidEffort: "ultra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := renderCodexAgentTemplate(t, content.TransformAgentForCodex(tt.source), tt.quality)
			assert.Contains(t, got, `model = "`+tt.wantModel+`"`)
			assert.Contains(t, got, `model_reasoning_effort = "`+tt.wantEffort+`"`)
			if tt.forbidEffort != "" {
				assert.NotContains(t, got, `model_reasoning_effort = "`+tt.forbidEffort+`"`)
			}
		})
	}
}

func TestTransformAgentForCodex_OmitsRuntimeDefaultProfile(t *testing.T) {
	t.Parallel()

	source := codexProfileSource("executor", "sonnet", "medium")
	got := renderCodexAgentTemplateData(t, content.TransformAgentForCodex(source), codexAgentRenderData{
		ProjectName: "test-project",
		IsFullMode:  true,
		OmitProfile: true,
	})

	assert.NotContains(t, got, "model =")
	assert.NotContains(t, got, "model_reasoning_effort =")
}

func codexProfileSource(name, model, effort string) content.AgentSource {
	return content.AgentSource{
		Meta: content.AgentSourceMeta{
			Name:        name,
			Description: "Codex profile test agent",
			Model:       model,
			Effort:      effort,
		},
	}
}

func renderCodexAgentTemplate(t *testing.T, source string, quality config.QualityConf) string {
	t.Helper()

	return renderCodexAgentTemplateData(t, source, codexAgentRenderData{
		ProjectName: "test-project",
		IsFullMode:  true,
		Quality:     quality,
	})
}

func renderCodexAgentTemplateData(t *testing.T, source string, data codexAgentRenderData) string {
	t.Helper()

	tmpl, err := template.New("agent").Parse(source)
	require.NoError(t, err)

	var output bytes.Buffer
	err = tmpl.Execute(&output, data)
	require.NoError(t, err)
	return output.String()
}

type codexAgentRenderData struct {
	ProjectName string
	IsFullMode  bool
	Quality     config.QualityConf
	OmitProfile bool
}

func (d codexAgentRenderData) CodexAgentModel(agentName, sourceTier, declaredEffort string) string {
	if d.OmitProfile {
		return ""
	}
	return d.Quality.CodexAgentProfile(agentName, sourceTier, declaredEffort).Model
}

func (d codexAgentRenderData) CodexAgentEffort(agentName, sourceTier, declaredEffort string) string {
	if d.OmitProfile {
		return ""
	}
	return d.Quality.CodexAgentEffort(agentName, sourceTier, declaredEffort)
}
