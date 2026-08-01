package omp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

// writeOMPConfig seeds .omp/config.yml with raw bytes and persists a harness
// config that lists omp, returning the config path.
func writeOMPConfig(t *testing.T, dir, contents string) string {
	t.Helper()

	cfgPath := filepath.Join(dir, configFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
	require.NoError(t, os.WriteFile(cfgPath, []byte(contents), 0o600))

	harness := config.DefaultFullConfig("omp-cfg")
	harness.Platforms = []string{"omp"}
	require.NoError(t, config.Save(dir, harness))
	return cfgPath
}

// TestOMPGenerate_RefusesMarkerInsideAnyDocumentScalar closes the multi-document
// hole. yaml.Unmarshal into a yaml.Node decodes only the FIRST document, so
// marker text sitting in a scalar of document 2+ slipped past the guard while
// the raw-text regex still matched it — and the rewrite then replaced a literal
// block scalar the user owned.
func TestOMPGenerate_RefusesMarkerInsideAnyDocumentScalar(t *testing.T) {
	t.Parallel()

	scalarDoc := "notes: |\n  " + markerBeginYml + "\n  CANARY-KEEP-THIS\n  " + markerEndYml + "\n"
	tests := map[string]string{
		"second document":      "model: keep-me\n---\n" + scalarDoc,
		"leading separator":    "---\nmodel: keep-me\n---\n" + scalarDoc,
		"empty first document": "---\n---\n" + scalarDoc,
		"third document":       "model: keep-me\n---\nother: value\n---\n" + scalarDoc,
	}

	for name, original := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cfgPath := writeOMPConfig(t, dir, original)

			_, genErr := NewWithRoot(dir).Generate(context.Background(), configForOMP())

			after, readErr := os.ReadFile(cfgPath)
			require.NoError(t, readErr)
			assert.Contains(t, string(after), "CANARY-KEEP-THIS",
				"a marker inside any document's scalar must never be rewritten away")
			if genErr != nil {
				assert.Equal(t, original, string(after),
					"failing closed must leave the file exactly as the user wrote it")
			}
		})
	}
}

// TestOMPGenerate_RefusesUnparseableConfig pins the companion rule: without a
// node tree there is no way to tell structure from content, so guessing is
// refused rather than risking the same data loss.
func TestOMPGenerate_RefusesUnparseableConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	original := "model: keep-me\n---\n\tthis: is not [valid yaml\n"
	cfgPath := writeOMPConfig(t, dir, original)

	_, genErr := NewWithRoot(dir).Generate(context.Background(), configForOMP())
	require.Error(t, genErr, "an unparseable document must fail closed")

	after, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.Equal(t, original, string(after))
}

// TestOMPClean_PreservesMarkerTextInsideUserScalar closes the Clean-path hole.
// stripManagedSection applied the structure-blind regex with no scalar guard, so
// `auto platform remove omp` destroyed a user value containing marker text —
// silently, since that path takes no backup and the caller discards the error.
func TestOMPClean_PreservesMarkerTextInsideUserScalar(t *testing.T) {
	t.Parallel()

	dir := generateOMPOnly(t)
	cfgPath := filepath.Join(dir, configFile)
	generated, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	userBlock := "notes: |\n  " + markerBeginYml + "\n  CANARY-KEEP-THIS\n  " + markerEndYml + "\n"
	require.NoError(t, os.WriteFile(cfgPath, append([]byte(userBlock), generated...), 0o600))

	// `auto platform remove omp` drops omp before Clean runs.
	remaining := config.DefaultFullConfig("omp-cfg")
	remaining.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(dir, remaining))

	_ = NewWithRoot(dir).Clean(context.Background())

	after, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr, "the config must not be deleted out from under the user value")
	assert.Contains(t, string(after), "CANARY-KEEP-THIS",
		"Clean must not strip through a marker that lives inside a user scalar")
}

// configForOMP returns a harness config listing only omp.
func configForOMP() *config.HarnessConfig {
	cfg := config.DefaultFullConfig("omp-cfg")
	cfg.Platforms = []string{"omp"}
	return cfg
}

// TestOMPGenerate_StillAcceptsOrdinaryConfig guards against the fail-closed
// checks degenerating into refusing everything.
func TestOMPGenerate_StillAcceptsOrdinaryConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := writeOMPConfig(t, dir, "disabledProviders:\n  - anthropic\nmodel: local\n")

	_, err := NewWithRoot(dir).Generate(context.Background(), configForOMP())
	require.NoError(t, err, "an ordinary config must still regenerate")

	after, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(after), "disabledProviders:")
	assert.Equal(t, 1, strings.Count(string(after), markerBeginYml))
}

// TestOMPGenerate_RefusesMarkerCommentNestedInCollection closes the nested-
// comment bypass. The scalar guard walks only node.Value, but marker text can
// arrive as a genuine YAML COMMENT attached to a node inside a collection —
// invisible to that walk, yet still a section boundary to the regex. The
// rewrite then replaced a real user value and left the file unparseable.
//
// The check must be position-aware, not a blanket comment ban: the adapter's own
// BEGIN/END are comments too, attached to a direct child of the root mapping.
func TestOMPGenerate_RefusesMarkerCommentNestedInCollection(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"block sequence": "notes:\n  - aaa\n" + markerBeginYml +
			"\n  - CANARY-KEEP-THIS\n" + markerEndYml + "\n  - zzz\nmodel: keep-me\n",
		"flow sequence": "notes: [\n  aaa,\n" + markerBeginYml +
			"\n  CANARY-KEEP-THIS,\n" + markerEndYml + "\n  zzz,\n]\nmodel: keep-me\n",
		"flow mapping": "notes: {\n  a: 1,\n" + markerBeginYml +
			"\n  keep: CANARY-KEEP-THIS,\n" + markerEndYml + "\n  z: 2,\n}\nmodel: keep-me\n",
		"root sequence": markerBeginYml + "\n- CANARY-KEEP-THIS\n" + markerEndYml + "\n",
	}

	for name, original := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cfgPath := writeOMPConfig(t, dir, original)

			_, genErr := NewWithRoot(dir).Generate(context.Background(), configForOMP())

			after, readErr := os.ReadFile(cfgPath)
			require.NoError(t, readErr)
			assert.Contains(t, string(after), "CANARY-KEEP-THIS",
				"a marker comment nested in a collection must never be treated as a section")
			require.Error(t, genErr, "the rewrite must fail closed")
			assert.Equal(t, original, string(after),
				"failing closed must leave the file exactly as the user wrote it")

			var parsed any
			assert.NoError(t, yaml.Unmarshal(after, &parsed),
				"the file must still parse; the old rewrite corrupted it")
		})
	}
}

// TestOMPClean_PreservesMarkerCommentNestedInCollection covers the same bypass
// on the Clean path, where there is no backup and the caller drops the error.
func TestOMPClean_PreservesMarkerCommentNestedInCollection(t *testing.T) {
	t.Parallel()

	dir := generateOMPOnly(t)
	cfgPath := filepath.Join(dir, configFile)
	generated, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	userBlock := "notes: [\n  aaa,\n" + markerBeginYml +
		"\n  CANARY-KEEP-THIS,\n" + markerEndYml + "\n  zzz,\n]\n"
	require.NoError(t, os.WriteFile(cfgPath, append([]byte(userBlock), generated...), 0o600))

	remaining := config.DefaultFullConfig("omp-cfg")
	remaining.Platforms = []string{"claude-code"}
	require.NoError(t, config.Save(dir, remaining))

	_ = NewWithRoot(dir).Clean(context.Background())

	after, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(after), "CANARY-KEEP-THIS",
		"Clean must not strip through a marker comment nested in a collection")
}

// TestOMPUpdate_ReplacesStaleManagedSection pins the upgrade path the
// position-aware design exists to protect. A guard that required the existing
// managed content to byte-equal the current render would refuse forever once the
// adapter's emission changed, so this fixes an OLD managed body and requires the
// update to replace it rather than reject it.
func TestOMPUpdate_ReplacesStaleManagedSection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	stale := "disabledProviders:\n  - anthropic\n\n" + markerBeginYml +
		"\nskills:\n  customDirectories:\n    - .agents/OLD-PATH\nlegacyKey: from-an-older-adapter\n" +
		markerEndYml + "\n"
	cfgPath := writeOMPConfig(t, dir, stale)

	adapterUnderTest := NewWithRoot(dir)
	_, err := adapterUnderTest.Generate(context.Background(), configForOMP())
	require.NoError(t, err, "a stale managed section must not block regeneration")
	_, err = adapterUnderTest.Update(context.Background(), configForOMP())
	require.NoError(t, err, "nor must it block update")

	after, readErr := os.ReadFile(cfgPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(after), "disabledProviders:", "user keys survive the upgrade")
	assert.Contains(t, string(after), ".agents/skills", "the current managed content is written")
	assert.NotContains(t, string(after), "OLD-PATH", "the stale managed content is replaced")
	assert.NotContains(t, string(after), "legacyKey", "stale managed keys do not linger")
	assert.Equal(t, 1, strings.Count(string(after), markerBeginYml))
}
