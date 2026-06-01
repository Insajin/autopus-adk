package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/domainreadiness"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestQACoverageCmd_SummarizesLatestIndexes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := domainreadiness.WriteStarterCatalog(dir, domainreadiness.DefaultCatalogPath)
	require.NoError(t, err)
	runPath := filepath.Join(dir, ".autopus", "qa", "runs", "r1", "run-index.json")
	releasePath := filepath.Join(dir, ".autopus", "qa", "releases", "rel1", "release-index.json")
	writeJSONFile(t, runPath, qarun.Index{
		SchemaVersion: qarun.RunIndexSchemaVersion,
		Status:        "passed",
		ManifestPaths: []string{".autopus/qa/manifests/fast.json"},
		Checks:        []qarun.IndexCheck{{ID: "fast", JourneyID: "node-fast", Adapter: "node-script", Status: "passed"}},
		AdapterResults: []qarun.AdapterResult{{
			Adapter:            "node-script",
			JourneyID:          "node-fast",
			Status:             "passed",
			QAMESHManifestPath: ".autopus/qa/manifests/fast.json",
		}},
	})
	writeJSONFile(t, releasePath, qarelease.Index{
		SchemaVersion: qarelease.IndexSchemaVersion,
		Status:        qarelease.GateStatusPassed,
		SelectedLanes: []string{"fast"},
		LaneRows: []qarelease.LaneRow{{
			Lane:          "fast",
			LanePolicy:    qarelease.LanePolicyMust,
			Status:        qarelease.LaneStatusPassed,
			LaneVerdict:   qarelease.LaneVerdictPass,
			RunIndexPath:  runPath,
			ManifestPaths: []string{".autopus/qa/manifests/fast.json"},
		}},
	})

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"coverage", "--project-dir", dir, "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa coverage")
	assert.Equal(t, "ok", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, "qamesh.coverage.v1", data["schema_version"])
	assert.Equal(t, "ready", data["status"])
	summary := data["summary"].(map[string]any)
	assert.Equal(t, 1, int(summary["lane_count"].(float64)))
	assert.Equal(t, 1, int(summary["journey_count"].(float64)))
	assert.Equal(t, 1, int(summary["manifest_count"].(float64)))
}

func TestQACoverageCmd_IsRegisteredUnderQA(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	coverageCmd, _, err := root.Find([]string{"qa", "coverage"})
	require.NoError(t, err)
	require.NotNil(t, coverageCmd)
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, append(body, '\n'), 0o644))
}
