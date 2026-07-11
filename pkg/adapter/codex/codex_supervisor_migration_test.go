package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestInspectLegacySupervisorModelMigratesKnownManagedProfiles(t *testing.T) {
	t.Parallel()

	profiles := []struct {
		model  string
		effort string
	}{
		{model: config.CodexLegacyModel, effort: config.CodexEffortXHigh},
		{model: config.CodexSolModel, effort: config.CodexEffortXHigh},
		{model: config.CodexSolModel, effort: config.CodexEffortUltra},
	}
	for _, profile := range profiles {
		profile := profile
		t.Run(profile.model+"/"+profile.effort, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			cfg := legacySupervisorInspectionConfig()
			writeManagedSupervisorConfig(t, dir, profile.model, profile.effort, "")

			inspection, err := InspectLegacySupervisorModel(dir, cfg)
			require.NoError(t, err)
			assert.True(t, inspection.HasProjectOverride)
			assert.True(t, inspection.Migratable)
			assert.False(t, inspection.UserOwned)
			assert.Equal(t, LegacySupervisorReasonManagedProfile, inspection.Reason)
		})
	}
}

func TestInspectLegacySupervisorModelPreservesAmbiguousOrOwnedOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		model      string
		effort     string
		marker     string
		mutate     func(*config.HarnessConfig, string)
		wantReason string
		userOwned  bool
	}{
		{
			name:       "ambiguous legacy effort",
			model:      config.CodexLegacyModel,
			effort:     config.CodexEffortMedium,
			wantReason: LegacySupervisorReasonCustomProfile,
			userOwned:  true,
		},
		{
			name:       "custom profile",
			model:      "gpt-5.4",
			effort:     config.CodexEffortHigh,
			wantReason: LegacySupervisorReasonCustomProfile,
			userOwned:  true,
		},
		{
			name:       "user marker",
			model:      config.CodexLegacyModel,
			effort:     config.CodexEffortXHigh,
			marker:     codexUserModelMarker + ": model, model_reasoning_effort",
			wantReason: LegacySupervisorReasonUserMarker,
			userOwned:  true,
		},
		{
			name:   "checksum drift",
			model:  config.CodexLegacyModel,
			effort: config.CodexEffortXHigh,
			mutate: func(_ *config.HarnessConfig, dir string) {
				path := filepath.Join(dir, codexConfigRelPath)
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(path, append(data, []byte("# local edit\n")...), 0o644))
			},
			wantReason: LegacySupervisorReasonChecksumDrift,
			userOwned:  true,
		},
		{
			name:   "explicit quality policy",
			model:  config.CodexLegacyModel,
			effort: config.CodexEffortXHigh,
			mutate: func(cfg *config.HarnessConfig, _ string) {
				cfg.Quality.SupervisorModelPolicy = config.SupervisorModelPolicyQuality
			},
			wantReason: LegacySupervisorReasonExplicitPolicy,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			cfg := legacySupervisorInspectionConfig()
			writeManagedSupervisorConfig(t, dir, tt.model, tt.effort, tt.marker)
			if tt.mutate != nil {
				tt.mutate(cfg, dir)
			}

			inspection, err := InspectLegacySupervisorModel(dir, cfg)
			require.NoError(t, err)
			assert.True(t, inspection.HasProjectOverride)
			assert.False(t, inspection.Migratable)
			assert.Equal(t, tt.userOwned, inspection.UserOwned)
			assert.Equal(t, tt.wantReason, inspection.Reason)
		})
	}
}

func TestInspectLegacySupervisorModelTreatsMissingManifestAsUserOwned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := legacySupervisorInspectionConfig()
	content := managedSupervisorConfigContent(config.CodexLegacyModel, config.CodexEffortXHigh, "")
	path := filepath.Join(dir, codexConfigRelPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	inspection, err := InspectLegacySupervisorModel(dir, cfg)
	require.NoError(t, err)
	assert.True(t, inspection.HasProjectOverride)
	assert.False(t, inspection.Migratable)
	assert.True(t, inspection.UserOwned)
	assert.Equal(t, LegacySupervisorReasonManifestMissing, inspection.Reason)
}

func TestInspectSupervisorOverrideOwnershipMatchesPerKeyMergeRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		model       string
		effort      string
		marker      string
		wantManaged bool
		wantUser    bool
	}{
		{
			name:        "known managed tuple",
			model:       config.CodexLegacyModel,
			effort:      config.CodexEffortXHigh,
			wantManaged: true,
		},
		{
			name:     "custom tuple",
			model:    "custom/model",
			effort:   config.CodexEffortHigh,
			wantUser: true,
		},
		{
			name:        "partially marked tuple",
			model:       config.CodexLegacyModel,
			effort:      config.CodexEffortXHigh,
			marker:      codexUserModelMarker + ": model",
			wantManaged: true,
			wantUser:    true,
		},
		{
			name:     "fully marked tuple",
			model:    config.CodexLegacyModel,
			effort:   config.CodexEffortXHigh,
			marker:   codexUserModelMarker,
			wantUser: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeManagedSupervisorConfig(t, dir, tt.model, tt.effort, tt.marker)

			inspection, err := InspectSupervisorOverrideOwnership(dir)
			require.NoError(t, err)
			assert.True(t, inspection.HasProjectOverride)
			assert.Equal(t, tt.wantManaged, inspection.HasManagedOverride)
			assert.Equal(t, tt.wantUser, inspection.HasUserOwnedOverride)
		})
	}
}

func legacySupervisorInspectionConfig() *config.HarnessConfig {
	cfg := config.DefaultFullConfig("legacy-project")
	cfg.Platforms = []string{"codex"}
	cfg.Quality.SupervisorModelPolicy = ""
	return cfg
}

func writeManagedSupervisorConfig(t *testing.T, dir, model, effort, marker string) {
	t.Helper()
	content := managedSupervisorConfigContent(model, effort, marker)
	path := filepath.Join(dir, codexConfigRelPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	manifest := adapter.NewManifest(adapterName)
	manifest.Files[codexConfigRelPath] = adapter.ManifestFile{
		Checksum: adapter.Checksum(content),
		Policy:   adapter.OverwriteMerge,
	}
	require.NoError(t, manifest.Save(dir))
}

func managedSupervisorConfigContent(model, effort, marker string) string {
	content := codexGeneratedConfigHeader + "\n"
	if marker != "" {
		content += marker + "\n"
	}
	return content + "model = \"" + model + "\"\n" +
		"model_reasoning_effort = \"" + effort + "\"\n" +
		"model_reasoning_summary = \"auto\"\n" +
		"model_verbosity = \"medium\"\n"
}
