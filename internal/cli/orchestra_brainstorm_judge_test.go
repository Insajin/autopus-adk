package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestSeparateBrainstormJudge_RemovesJudgeFamilyFromDebaters(t *testing.T) {
	t.Parallel()

	providers := []orchestra.ProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "codex", Binary: "codex"},
		{Name: "gemini", Binary: "gemini"},
	}

	got, family, err := separateBrainstormJudge(providers, "claude")

	require.NoError(t, err)
	assert.Equal(t, "anthropic", family)
	assert.Equal(t, []string{"codex", "gemini"}, providerConfigNames(got))
}

func TestSeparateBrainstormJudge_FailsClosedForUnknownFamily(t *testing.T) {
	t.Parallel()

	_, _, err := separateBrainstormJudge([]orchestra.ProviderConfig{
		{Name: "codex", Binary: "codex"},
		{Name: "gemini", Binary: "gemini"},
	}, "custom-judge")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "model family")
}

func TestSeparateBrainstormJudge_FailsClosedWithFewerThanTwoDebaters(t *testing.T) {
	t.Parallel()

	_, _, err := separateBrainstormJudge([]orchestra.ProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "codex", Binary: "codex"},
	}, "claude")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least two")
}

func TestResolveBrainstormJudgeConfig_PreservesRuntimeConfigAndSetsFamily(t *testing.T) {
	t.Parallel()

	judge, err := resolveBrainstormJudgeConfig([]orchestra.ProviderConfig{{
		Name: "claude", Binary: "/custom/claude", Args: []string{"--print"},
	}}, nil, "brainstorm", "claude", "anthropic", "", "")

	require.NoError(t, err)
	require.NotNil(t, judge)
	assert.Equal(t, "/custom/claude", judge.Binary)
	assert.Equal(t, "anthropic", judge.ModelFamily)
}
