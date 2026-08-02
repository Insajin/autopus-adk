package config

// SPEC-STICKYRULE-001 cadence configuration oracles for REQ-STICKYRULE-MAP-01.
//
// Planned surface: HooksConf.StickyCadence int `yaml:"sticky_cadence,omitempty"`.
//
// A non-integer scalar is rejected by the typed yaml.Unmarshal in loader.go
// before the cadence is ever read, so it is a config load error rather than a
// fallback case. That distinction is what keeps the fallback rule total over
// exactly three inputs: absent, zero, and negative.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeStickyCadenceConfig writes a minimal valid autopus.yaml carrying the
// supplied hooks block and returns its directory.
func writeStickyCadenceConfig(t *testing.T, hooksBlock string) string {
	t.Helper()
	dir := t.TempDir()
	content := "mode: full\nproject_name: sticky\nplatforms:\n  - claude-code\n" + hooksBlock
	require.NoError(t, os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(content), 0o644))
	return dir
}

// TestLoad_StickyCadenceIsReadFromHooks pins the configured value.
func TestLoad_StickyCadenceIsReadFromHooks(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeStickyCadenceConfig(t, "hooks:\n  sticky_cadence: 2\n"))

	require.NoError(t, err)
	assert.Equal(t, 2, cfg.Hooks.StickyCadence)
}

// TestLoad_StickyCadenceAbsentDecodesToZero fixes the input the resolver turns
// into the default of 8.
func TestLoad_StickyCadenceAbsentDecodesToZero(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeStickyCadenceConfig(t, "hooks:\n  pre_commit_arch: true\n"))

	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Hooks.StickyCadence,
		"an absent key decodes to the zero value, which resolves to the default cadence")
}

// TestLoad_NegativeStickyCadenceLoadsAndDefersToTheResolver keeps a negative
// value a load-time success: it is the resolver, not the loader, that maps it
// to the default.
func TestLoad_NegativeStickyCadenceLoadsAndDefersToTheResolver(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeStickyCadenceConfig(t, "hooks:\n  sticky_cadence: -3\n"))

	require.NoError(t, err)
	assert.Equal(t, -3, cfg.Hooks.StickyCadence)
}

// TestStickyCadence_NormalizesUnsetAndNonPositiveValues covers the accessor the
// schema field documents as the only supported read path. The tests above assert
// what the loader decodes; this one asserts what callers actually receive, which
// is where the "unset" contract ResolveStickyCadence depends on is established.
func TestStickyCadence_NormalizesUnsetAndNonPositiveValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *HarnessConfig
		want int
	}{
		{name: "nil config", cfg: nil, want: 0},
		{name: "absent key decodes to zero", cfg: &HarnessConfig{}, want: 0},
		{
			name: "negative reports unset",
			cfg:  &HarnessConfig{Hooks: HooksConf{StickyCadence: -3}},
			want: 0,
		},
		{
			name: "positive is reported verbatim",
			cfg:  &HarnessConfig{Hooks: HooksConf{StickyCadence: 2}},
			want: 2,
		},
		{
			name: "one is a positive value, not a sentinel",
			cfg:  &HarnessConfig{Hooks: HooksConf{StickyCadence: 1}},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, StickyCadence(tt.cfg))
		})
	}
}

// TestDefaultFullConfig_LeavesStickyCadenceUnset keeps the cadence default
// stated in exactly one place. A workspace with no autopus.yaml loads
// DefaultFullConfig, so a non-zero value here would silently override
// rulecond.ResolveStickyCadence and give two defaults that can drift apart.
func TestDefaultFullConfig_LeavesStickyCadenceUnset(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, StickyCadence(DefaultFullConfig("sticky")),
		"the shipped default config must defer the cadence to the resolver")
}

// TestLoad_NonIntegerStickyCadenceIsAConfigLoadError is the S14 type-error row.
func TestLoad_NonIntegerStickyCadenceIsAConfigLoadError(t *testing.T) {
	t.Parallel()

	cfg, err := Load(writeStickyCadenceConfig(t, "hooks:\n  sticky_cadence: fast\n"))

	require.Error(t, err,
		"a non-integer scalar must fail config load rather than silently falling back")
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "config",
		"the failure must be reported as a config problem, got %q", err.Error())
}
