package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQAReadinessCmd_ProjectsReadOnlyRedactedJSONEnvelope(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("..", "..", "testdata", "qa", "readiness", "non_autopus_fixture")
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"qa", "readiness",
		"--workspace-root", fixture,
		"--repo-root", filepath.Join(fixture, "repos", "portable-shop"),
		"--workspace-id", "fixture-workspace",
		"--repo-id", "portable-shop",
		"--run-index", filepath.Join(fixture, "qa", "run-index.json"),
		"--release-index", filepath.Join(fixture, "qa", "release-index.json"),
		"--format", "json",
	})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "auto qa readiness")
	assert.Equal(t, "ok", payload["status"])

	data := payload["data"].(map[string]any)
	assert.Equal(t, "qamesh.readiness_projection.v1", data["schema_version"])
	assert.Equal(t, "blocked", data["release_verdict"])
	assert.Equal(t, false, data["raw_payload_present"])
	assert.Equal(t, "autopus-adk", data["contract_owner"])
	assert.NotContains(t, data, "reference_consumers")

	statuses := data["lane_statuses"].(map[string]any)
	assert.Equal(t, "passed", statuses["fast"])
	assert.Equal(t, "failed", statuses["browser-staging"])
	assert.Equal(t, "blocked", statuses["desktop-native"])
	assert.Equal(t, "skipped", statuses["gui-explore"])
	assert.Equal(t, "deferred", statuses["mobile-readiness"])
	assert.Equal(t, "setup_gap", statuses["canary-explicit"])

	action := data["feedback_actions"].([]any)[0].(map[string]any)
	assert.Equal(t, true, action["enabled"])
	assert.Equal(t, "auto qa feedback --to codex --evidence qa/evidence/manifests/login.json", action["command_display"])
	assert.NotContains(t, out.String(), "/Users/")
	assert.NotContains(t, out.String(), "raw_network")
	assert.NotContains(t, out.String(), "provider_payload")
}
