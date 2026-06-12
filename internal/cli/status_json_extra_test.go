package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeSpecStatus_Variants verifies underscore/case normalization.
func TestNormalizeSpecStatus_Variants(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"  Done ":     "done",
		"IN_PROGRESS": "in-progress",
		"Completed":   "completed",
		"in_progress": "in-progress",
		"":            "",
		"Approved\t":  "approved",
	}
	for in, want := range cases {
		assert.Equal(t, want, normalizeSpecStatus(in), "input %q", in)
	}
}

// TestStatusCheckID_WithAndWithoutModule verifies check ID composition.
func TestStatusCheckID_WithAndWithoutModule(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "spec.SPEC-1", statusCheckID(specEntry{id: "SPEC-1"}))
	assert.Equal(t, "spec.mod.SPEC-2", statusCheckID(specEntry{id: "SPEC-2", module: "mod"}))
}

// TestRelativeStatusPath_Cases verifies relative path resolution and fallbacks.
func TestRelativeStatusPath_Cases(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", relativeStatusPath("/base", ""))
	got := relativeStatusPath("/base", "/base/a/spec.md")
	assert.Equal(t, "a/spec.md", got)
}

// TestBuildStatusJSONPayload_Empty verifies the no-specs warn envelope.
func TestBuildStatusJSONPayload_Empty(t *testing.T) {
	t.Parallel()

	data, warnings, checks, status := buildStatusJSONPayload("/base", nil)
	assert.Equal(t, jsonStatusWarn, status)
	assert.Equal(t, 0, data.Summary.Total)
	require.Len(t, warnings, 1)
	assert.Equal(t, "no_specs", warnings[0].Code)
	require.Len(t, checks, 1)
	assert.Equal(t, "specs.scan", checks[0].ID)
}

// TestBuildStatusJSONPayload_AllDone verifies summary counts when every SPEC is done.
func TestBuildStatusJSONPayload_AllDone(t *testing.T) {
	t.Parallel()

	specs := []specEntry{
		{id: "SPEC-A", status: "completed", path: "/base/.autopus/specs/SPEC-A/spec.md"},
		{id: "SPEC-B", status: "implemented", module: "m"},
	}
	data, warnings, checks, status := buildStatusJSONPayload("/base", specs)
	assert.Equal(t, jsonStatusOK, status)
	assert.Equal(t, 2, data.Summary.Total)
	assert.Equal(t, 2, data.Summary.Done)
	assert.Equal(t, 0, data.Summary.Open)
	assert.Nil(t, warnings)
	require.Len(t, checks, 2)
	assert.Equal(t, "pass", checks[0].Status)
	// First spec carries a relative source path.
	assert.Equal(t, ".autopus/specs/SPEC-A/spec.md", data.Specs[0].Source)
	assert.True(t, data.Specs[0].Completed)
}

// TestBuildStatusJSONPayload_OpenAndDraft verifies open findings and draft warning.
func TestBuildStatusJSONPayload_OpenAndDraft(t *testing.T) {
	t.Parallel()

	specs := []specEntry{
		{id: "SPEC-D", status: "draft", module: "mod"},
		{id: "SPEC-P", status: "in_progress"},
		{id: "SPEC-A", status: "approved"},
	}
	data, warnings, checks, status := buildStatusJSONPayload("/base", specs)
	assert.Equal(t, jsonStatusWarn, status)
	assert.Equal(t, 3, data.Summary.Open)
	assert.Equal(t, 1, data.Summary.Draft)
	assert.Equal(t, 1, data.Summary.InProgress)
	assert.Equal(t, 1, data.Summary.Approved)
	require.Len(t, data.Findings, 3)
	assert.Equal(t, "open_spec", data.Findings[0].Code)
	// Warnings include both open and draft codes.
	codes := map[string]bool{}
	for _, w := range warnings {
		codes[w.Code] = true
	}
	assert.True(t, codes["open_specs"])
	assert.True(t, codes["draft_specs"])
	require.Len(t, checks, 3)
	assert.Equal(t, "warn", checks[0].Status)
}

// TestBuildStatusWarnings_NoOpen returns nil when nothing is open.
func TestBuildStatusWarnings_NoOpen(t *testing.T) {
	t.Parallel()

	assert.Nil(t, buildStatusWarnings(statusJSONSummary{Open: 0}, nil))
}

// TestBuildStatusWarnings_OpenNoDraft yields only the open message.
func TestBuildStatusWarnings_OpenNoDraft(t *testing.T) {
	t.Parallel()

	msgs := buildStatusWarnings(statusJSONSummary{Open: 2}, nil)
	require.Len(t, msgs, 1)
	assert.Equal(t, "open_specs", msgs[0].Code)
	assert.Contains(t, msgs[0].Message, "2 SPEC(s) remain open")
}

// TestRunStatusJSON_EmitsEnvelope drives runStatusJSON over a real temp tree.
func TestRunStatusJSON_EmitsEnvelope(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	specDir := filepath.Join(base, ".autopus", "specs", "SPEC-XYZ-001")
	require.NoError(t, os.MkdirAll(specDir, 0o755))
	content := "# SPEC-XYZ-001: Demo\n\n**Status**: draft\n"
	require.NoError(t, os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644))

	cmd := newStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--dir", base, "--json"})
	require.NoError(t, cmd.Execute())

	var env jsonEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.Equal(t, jsonStatusWarn, env.Status)
	assert.Equal(t, cliJSONSchemaVersion, env.SchemaVersion)

	// Data must carry the scanned spec with normalized draft status.
	raw, err := json.Marshal(env.Data)
	require.NoError(t, err)
	var data statusJSONData
	require.NoError(t, json.Unmarshal(raw, &data))
	require.Len(t, data.Specs, 1)
	assert.Equal(t, "SPEC-XYZ-001", data.Specs[0].ID)
	assert.Equal(t, "draft", data.Specs[0].Status)
	assert.False(t, data.Specs[0].Completed)
}
