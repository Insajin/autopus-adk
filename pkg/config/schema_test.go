package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessConfig_Validate_Valid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  HarnessConfig
	}{
		{
			name: "full mode",
			cfg: HarnessConfig{
				Mode:        ModeFull,
				ProjectName: "test-project",
				Platforms:   []string{"claude-code"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			require.NoError(t, err)
		})
	}
}

func TestHarnessConfig_Validate_Invalid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     HarnessConfig
		wantErr string
	}{
		{
			name:    "invalid mode",
			cfg:     HarnessConfig{Mode: "invalid", ProjectName: "p", Platforms: []string{"claude-code"}},
			wantErr: "invalid mode",
		},
		{
			name:    "empty project name",
			cfg:     HarnessConfig{Mode: ModeFull, ProjectName: "", Platforms: []string{"claude-code"}},
			wantErr: "project_name is required",
		},
		{
			name:    "no platforms",
			cfg:     HarnessConfig{Mode: ModeFull, ProjectName: "p", Platforms: []string{}},
			wantErr: "at least one platform",
		},
		{
			name:    "invalid platform",
			cfg:     HarnessConfig{Mode: ModeFull, ProjectName: "p", Platforms: []string{"invalid"}},
			wantErr: "invalid platform",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestHarnessConfig_ModeHelpers(t *testing.T) {
	t.Parallel()
	full := HarnessConfig{Mode: ModeFull}
	assert.True(t, full.IsFullMode())
}

func TestHarnessConfig_Validate_QualityDefaultExists(t *testing.T) {
	t.Parallel()
	// When Quality.Default is non-empty and the named preset exists, Validate should pass.
	cfg := HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Quality: QualityConf{
			Default: "fast",
			Presets: map[string]QualityPreset{
				"fast": {Agents: map[string]string{"planner": "haiku"}},
			},
		},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestHarnessConfig_Validate_QualityDefaultNotInPresets(t *testing.T) {
	t.Parallel()
	// When Quality.Default names a preset that does not exist, Validate should return an error.
	cfg := HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Quality: QualityConf{
			Default: "nonexistent",
			Presets: map[string]QualityPreset{
				"fast": {Agents: map[string]string{"planner": "haiku"}},
			},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality.default")
}

func TestHarnessConfig_Validate_QualityDefaultEmpty(t *testing.T) {
	t.Parallel()
	// Zero-value Quality (empty Default, nil Presets) should be valid — no preset lookup occurs.
	cfg := HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Quality:     QualityConf{},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestHarnessConfig_Validate_QualityInvalidModelTier(t *testing.T) {
	t.Parallel()
	// When a preset agent is mapped to an unknown model tier, Validate should return an error.
	cfg := HarnessConfig{
		Mode:        ModeFull,
		ProjectName: "test-project",
		Platforms:   []string{"claude-code"},
		Quality: QualityConf{
			Default: "fast",
			Presets: map[string]QualityPreset{
				"fast": {Agents: map[string]string{
					"planner":  "haiku",
					"executor": "invalid-tier",
				}},
			},
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown model tier")
}

func TestUsageProfile_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		profile UsageProfile
		want    bool
	}{
		{"", true},
		{ProfileDeveloper, true},
		{ProfileFullstack, true},
		{"invalid", false},
		{"DEVELOPER", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.profile.IsValid())
		})
	}
}

func TestUsageProfile_Effective(t *testing.T) {
	t.Parallel()
	tests := []struct {
		profile UsageProfile
		want    UsageProfile
	}{
		{"", ProfileDeveloper},
		{ProfileDeveloper, ProfileDeveloper},
		{ProfileFullstack, ProfileFullstack},
	}
	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.profile.Effective())
		})
	}
}

func TestHintsConf_IsPlatformHintEnabled(t *testing.T) {
	t.Parallel()
	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name string
		conf HintsConf
		want bool
	}{
		{"nil (default enabled)", HintsConf{Platform: nil}, true},
		{"explicit true", HintsConf{Platform: boolPtr(true)}, true},
		{"explicit false", HintsConf{Platform: boolPtr(false)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.conf.IsPlatformHintEnabled())
		})
	}
}

func TestHarnessConfig_Validate_UsageProfile(t *testing.T) {
	t.Parallel()
	cfg := HarnessConfig{
		Mode:         ModeFull,
		ProjectName:  "test",
		Platforms:    []string{"claude-code"},
		UsageProfile: "invalid-profile",
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid usage_profile")
}

func TestProviderEntry_PromptViaArgs(t *testing.T) {
	t.Parallel()

	t.Run("default value is false", func(t *testing.T) {
		t.Parallel()
		p := ProviderEntry{Binary: "claude", Args: []string{"--print"}}
		assert.False(t, p.PromptViaArgs)
	})

	t.Run("can be set to true", func(t *testing.T) {
		t.Parallel()
		p := ProviderEntry{Binary: "gemini", Args: []string{}, PromptViaArgs: true}
		assert.True(t, p.PromptViaArgs)
	})

	t.Run("yaml deserialization preserves PromptViaArgs true", func(t *testing.T) {
		t.Parallel()
		conf := OrchestraConf{
			Providers: map[string]ProviderEntry{
				"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
				"claude": {Binary: "claude", Args: []string{"--print"}},
			},
		}
		assert.True(t, conf.Providers["gemini"].PromptViaArgs)
		assert.False(t, conf.Providers["claude"].PromptViaArgs)
	})
}
