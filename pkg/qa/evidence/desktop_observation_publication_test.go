package evidence

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopObservationPublication_WalksEntireRootWithExactRecursiveAllowlist(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	observation := successfulObservationEvidence(t)
	manifest := desktopObservationManifest(t, dir, observation, "passed")
	output := filepath.Join(dir, "published")
	manifestPath, err := WriteFinalManifest(manifest, output)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(output, "manifest.json"), manifestPath)

	files := []string{}
	err = filepath.WalkDir(output, func(path string, entry fs.DirEntry, walkErr error) error {
		require.NoError(t, walkErr)
		if entry.IsDir() {
			return nil
		}
		relative, relErr := filepath.Rel(output, path)
		require.NoError(t, relErr)
		files = append(files, filepath.ToSlash(relative))
		body, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assertForbiddenDesktopInventoryZero(t, body)
		assertPublishedObservationAllowlist(t, relative, body)
		return nil
	})
	require.NoError(t, err)
	sort.Strings(files)
	assert.Equal(t, []string{
		"artifacts/desktop_observation/desktop-observation.json",
		"manifest.json",
	}, files)
}

func TestDesktopObservationPublication_NestedUnknownAndDuplicateFieldsFailClosed(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(successfulObservationEvidence(t))
	require.NoError(t, err)
	tests := []struct {
		name        string
		old         string
		replacement string
		want        error
	}{
		{name: "projection", old: `"provider_ref":"provider-local"`, replacement: `"provider_ref":"provider-local","raw_handle":"0x42"`, want: desktopobserve.ErrUnknownField},
		{name: "node", old: `"role":"AXApplication"`, replacement: `"role":"AXApplication","index":7`, want: desktopobserve.ErrUnknownField},
		{name: "semantic state", old: `"enabled":true`, replacement: `"enabled":true,"raw_value":"secret"`, want: desktopobserve.ErrUnknownField},
		{name: "frame", old: `"x":0`, replacement: `"x":0,"screen_x":99`, want: desktopobserve.ErrUnknownField},
		{name: "provider", old: `"name":"autopus-desktop-local"`, replacement: `"name":"autopus-desktop-local","helper_path":"/tmp/helper"`, want: desktopobserve.ErrUnknownField},
		{name: "capability", old: `"name":"capabilities"`, replacement: `"name":"capabilities","raw_action":"press"`, want: desktopobserve.ErrUnknownField},
		{name: "check", old: `"id":"desktop-semantic-landmarks"`, replacement: `"id":"desktop-semantic-landmarks","error_text":"provider secret"`, want: desktopobserve.ErrUnknownField},
		{name: "redaction", old: `"redaction":{"status":"applied"}`, replacement: `"redaction":{"status":"applied","raw_payload":true}`, want: desktopobserve.ErrUnknownField},
		{name: "quarantine", old: `"quarantine":{"status":"cleared"}`, replacement: `"quarantine":{"status":"cleared","raw_path":"/Users/alice/raw"}`, want: desktopobserve.ErrUnknownField},
		{name: "duplicate nested key", old: `"provider_ref":"provider-local"`, replacement: `"provider_ref":"provider-local","provider_ref":"other"`, want: desktopobserve.ErrDuplicateKey},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			mutated := bytes.Replace(body, []byte(test.old), []byte(test.replacement), 1)
			require.NotEqual(t, body, mutated)
			_, err := desktopobserve.DecodeObservationEvidence(mutated)
			require.Error(t, err)
			assert.ErrorIs(t, err, test.want)
		})
	}
}

func assertPublishedObservationAllowlist(t *testing.T, relative string, body []byte) {
	t.Helper()
	var root map[string]any
	require.NoError(t, json.Unmarshal(body, &root))
	observation := root
	if filepath.Base(relative) == "manifest.json" {
		oracleResults := requireObject(t, root, "oracle_results")
		observation = requireObject(t, oracleResults, "desktop_observation")
	}
	assertObjectKeys(t, observation, []string{"semantic_projection", "deterministic_checks", "runtime_receipt"})
	projection := requireObject(t, observation, "semantic_projection")
	assertObjectKeys(t, projection, []string{"schema_version", "provider_ref", "app_ref", "window_ref", "state_ref", "digest", "root"})
	assertSemanticNodeAllowlist(t, requireObject(t, projection, "root"))

	checks := requireArray(t, observation, "deterministic_checks")
	for _, item := range checks {
		check := item.(map[string]any)
		assertAllowedObjectKeys(t, check, []string{"id", "status", "reason_code"})
		assertRequiredObjectKeys(t, check, []string{"id", "status"})
	}
	receipt := requireObject(t, observation, "runtime_receipt")
	assertObjectKeys(t, receipt, []string{"schema_version", "provider", "scope", "capability_summary", "reason_code", "next_step", "redaction", "quarantine"})
	assertObjectKeys(t, requireObject(t, receipt, "provider"), []string{"name", "version", "protocol_version"})
	assertObjectKeys(t, requireObject(t, receipt, "scope"), []string{"kind", "public_ref"})
	for _, item := range requireArray(t, receipt, "capability_summary") {
		assertObjectKeys(t, item.(map[string]any), []string{"name", "status"})
	}
	assertObjectKeys(t, requireObject(t, receipt, "redaction"), []string{"status"})
	assertObjectKeys(t, requireObject(t, receipt, "quarantine"), []string{"status"})
}

func assertSemanticNodeAllowlist(t *testing.T, node map[string]any) {
	t.Helper()
	assertAllowedObjectKeys(t, node, []string{"node_ref", "role", "name", "semantic_state", "frame", "advertised_actions", "children"})
	assertRequiredObjectKeys(t, node, []string{"node_ref", "role", "name", "semantic_state"})
	state := requireObject(t, node, "semantic_state")
	assertAllowedObjectKeys(t, state, []string{"enabled", "focused", "selected", "expanded", "checked", "value"})
	if frame, ok := node["frame"].(map[string]any); ok {
		assertObjectKeys(t, frame, []string{"x", "y", "width", "height"})
	}
	if children, ok := node["children"].([]any); ok {
		for _, child := range children {
			assertSemanticNodeAllowlist(t, child.(map[string]any))
		}
	}
}

func assertObjectKeys(t *testing.T, object map[string]any, allowed []string) {
	t.Helper()
	assertAllowedObjectKeys(t, object, allowed)
	assertRequiredObjectKeys(t, object, allowed)
}

func assertAllowedObjectKeys(t *testing.T, object map[string]any, allowed []string) {
	t.Helper()
	allowedSet := map[string]bool{}
	for _, key := range allowed {
		allowedSet[key] = true
	}
	for key := range object {
		assert.True(t, allowedSet[key], "unknown published key %q", key)
	}
}

func assertRequiredObjectKeys(t *testing.T, object map[string]any, required []string) {
	t.Helper()
	for _, key := range required {
		assert.Contains(t, object, key)
	}
}

func requireObject(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := object[key].(map[string]any)
	require.True(t, ok, "%s is not an object", key)
	return value
}

func requireArray(t *testing.T, object map[string]any, key string) []any {
	t.Helper()
	value, ok := object[key].([]any)
	require.True(t, ok, "%s is not an array", key)
	return value
}
