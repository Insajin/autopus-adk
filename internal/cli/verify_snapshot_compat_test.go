package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotProofReporter_PublicPlaywrightContract_AvoidsPrivateInternals(t *testing.T) {
	t.Parallel()

	// Given
	source := snapshotProofReporterSource()

	// Then
	assert.Contains(t, source, "projectSuite.project()")
	assert.Contains(t, source, "typeof projectSuite.project === 'function'")
	assert.Contains(t, source, "Array.isArray(suite.suites) ? suite.suites : []")
	assert.Contains(t, source, "projectSuite.title")
	assert.Contains(t, source, "ignoreSnapshots")
	assert.Contains(t, source, "config.version")
	assert.Contains(t, source, "config.updateSnapshots")
	assert.Contains(t, source, "project.dependencies")
	assert.Contains(t, source, "project.teardown")
	assert.Contains(t, source, "support_only")
	assert.Contains(t, source, "'unavailable'")
	assert.Contains(t, source, "'missing'")
	assert.NotContains(t, source, "_fullProject")
	assert.Contains(t, source, "version: 2")
}

func TestDecodeSnapshotComparisonProof_ValidV2Contracts_AcceptsPublicAndUnsupportedStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		contains []string
	}{
		{
			name: "playwright 1.59 public project contract",
			raw: `{
				"version": 2,
				"nonce": "nonce-159",
				"playwright_version": "1.59.1",
				"update_snapshots": "none",
				"projects": [{
					"name": "chromium",
					"ignore_snapshots": false,
					"state": "enabled",
					"source": "public"
				}]
			}`,
			contains: []string{`"playwright_version":"1.59.1"`, `"update_snapshots":"none"`, `"ignore_snapshots":false`, `"state":"enabled"`},
		},
		{
			name: "playwright 1.58 unsupported public field",
			raw: `{
				"version": 2,
				"nonce": "nonce-158",
				"playwright_version": "1.58.2",
				"update_snapshots": "none",
				"projects": [{
					"name": "chromium",
					"ignore_snapshots": null,
					"state": "unproven",
					"source": "unsupported"
				}]
			}`,
			contains: []string{`"playwright_version":"1.58.2"`, `"update_snapshots":"none"`, `"state":"unproven"`, `"source":"unsupported"`},
		},
		{
			name: "legacy reporter surface without public config fields",
			raw: `{
				"version": 2,
				"nonce": "nonce-legacy",
				"playwright_version": "unavailable",
				"update_snapshots": "missing",
				"projects": [{
					"name": "legacy suite [unsupported:0]",
					"ignore_snapshots": null,
					"state": "unproven",
					"source": "unsupported"
				}]
			}`,
			contains: []string{`"playwright_version":"unavailable"`, `"update_snapshots":"missing"`, `"state":"unproven"`},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// When
			proof, err := decodeSnapshotComparisonProof([]byte(test.raw))

			// Then
			require.NoError(t, err)
			encoded, err := json.Marshal(proof)
			require.NoError(t, err)
			assert.Contains(t, string(encoded), `"version":2`)
			for _, fragment := range test.contains {
				assert.Contains(t, string(encoded), fragment)
			}
		})
	}
}

func TestDecodeSnapshotComparisonProof_InconsistentV2Contracts_RejectsProof(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "enabled while snapshots are ignored",
			raw:  `{"version":2,"nonce":"n","playwright_version":"1.59.1","update_snapshots":"none","projects":[{"name":"chromium","ignore_snapshots":true,"state":"enabled","source":"public"}]}`,
		},
		{
			name: "enabled while snapshots may be updated",
			raw:  `{"version":2,"nonce":"n","playwright_version":"1.59.1","update_snapshots":"all","projects":[{"name":"chromium","ignore_snapshots":false,"state":"enabled","source":"public"}]}`,
		},
		{
			name: "unknown state",
			raw:  `{"version":2,"nonce":"n","playwright_version":"1.59.1","update_snapshots":"none","projects":[{"name":"chromium","ignore_snapshots":false,"state":"maybe","source":"public"}]}`,
		},
		{
			name: "duplicate required project",
			raw:  `{"version":2,"nonce":"n","playwright_version":"1.59.1","update_snapshots":"none","required_projects":["chromium","chromium"],"projects":[{"name":"chromium","ignore_snapshots":false,"state":"enabled","source":"public"}]}`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// When
			_, err := decodeSnapshotComparisonProof([]byte(test.raw))

			// Then
			assert.Error(t, err)
		})
	}
}

func TestRunPlaywright_DefaultInvocation_ForcesSnapshotUpdatesOff(t *testing.T) {
	// Given / When
	run := runPlaywrightWithCapturedArgs(t, "desktop", "")

	// Then
	assert.Contains(t, run.Args, "--update-snapshots=none")
	assert.Equal(t, 1, countExactArgument(run.Args, "--update-snapshots=none"))
}

func TestCollectJSONVisualEvidence_AnnotatedV2Proof_PreservesProofForVisualGate(t *testing.T) {
	t.Parallel()

	// Given
	output := []byte(`{
		"_autopusSnapshotComparisonProof": {
			"version": 2,
			"nonce": "nonce",
			"playwright_version": "1.59.1",
			"update_snapshots": "none",
			"projects": [{
				"name": "proof-disabled-project",
				"ignore_snapshots": true,
				"state": "disabled",
				"source": "public"
			}]
		},
		"suites": [{"specs": [{"tests": [{"results": [{"attachments": [{
			"name": "screenshot",
			"contentType": "image/png",
			"path": "test-results/fallback.png"
		}]}]}]}]}]
	}`)

	// When
	evidence := collectJSONVisualEvidence(output)

	// Then
	require.Len(t, evidence.Artifacts, 1)
	encoded, err := json.Marshal(evidence)
	require.NoError(t, err)
	publicEvidence := strings.ToLower(string(encoded))
	assert.Contains(t, publicEvidence, "snapshot")
	assert.Contains(t, publicEvidence, "disabled")
	assert.Contains(t, publicEvidence, "proof-disabled-project")
}

func countExactArgument(arguments []string, expected string) int {
	count := 0
	for _, argument := range arguments {
		if argument == expected {
			count++
		}
	}
	return count
}
