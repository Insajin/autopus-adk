package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQADomainReadinessCmd_IsRegistered(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()

	group, _, err := root.Find([]string{"qa", "domain-readiness"})
	require.NoError(t, err)
	require.NotNil(t, group)

	for _, name := range []string{"init", "plan", "report"} {
		sub, _, err := root.Find([]string{"qa", "domain-readiness", name})
		require.NoError(t, err)
		require.NotNil(t, sub)
	}
}

func TestQADomainReadinessInitPlanAndReportUseProjectLocalCatalog(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	catalogPath := filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json")

	initCmd := newQACmd()
	var initOut bytes.Buffer
	initCmd.SetOut(&initOut)
	initCmd.SetArgs([]string{"domain-readiness", "init", "--project-dir", dir, "--format", "json"})
	require.NoError(t, initCmd.Execute())
	assert.FileExists(t, catalogPath)

	initPayload := decodeJSONMap(t, initOut.Bytes())
	assertCommonJSONEnvelope(t, initPayload, "qa domain-readiness init")
	assert.Equal(t, "qamesh.domain_readiness_catalog.v1", initPayload["data"].(map[string]any)["schema_version"])

	planCmd := newQACmd()
	var planOut bytes.Buffer
	planCmd.SetOut(&planOut)
	planCmd.SetArgs([]string{"domain-readiness", "plan", "--project-dir", dir, "--lane", "fast", "--format", "json"})
	require.NoError(t, planCmd.Execute())

	planPayload := decodeJSONMap(t, planOut.Bytes())
	assertCommonJSONEnvelope(t, planPayload, "qa domain-readiness plan")
	plan := planPayload["data"].(map[string]any)
	assert.Equal(t, "qamesh.domain_readiness_plan.v1", plan["schema_version"])
	assert.Equal(t, false, plan["commands_executed"])
	assert.Equal(t, float64(1), plan["scenario_count"])
	assert.Equal(t, true, plan["validation"].(map[string]any)["valid"])

	reportCmd := newQACmd()
	var reportOut bytes.Buffer
	reportCmd.SetOut(&reportOut)
	reportCmd.SetArgs([]string{
		"domain-readiness", "report",
		"--project-dir", dir,
		"--run-id", "run-domain-readiness",
		"--workspace-id", "workspace-1",
		"--format", "json",
	})
	require.NoError(t, reportCmd.Execute())

	reportPayload := decodeJSONMap(t, reportOut.Bytes())
	assertCommonJSONEnvelope(t, reportPayload, "qa domain-readiness report")
	report := reportPayload["data"].(map[string]any)
	assert.Equal(t, "qamesh.domain_readiness_report.v1", report["schema_version"])
	assert.Equal(t, float64(1), report["evidence_count"])
	rows := report["evidence"].([]any)
	require.Len(t, rows, 1)
	first := rows[0].(map[string]any)
	assert.Equal(t, "domain_readiness_evidence.v1", first["schema_version"])
	assert.Equal(t, false, first["denominator_included"])
	assert.Equal(t, false, first["raw_payload_present"])
	assert.Equal(t, float64(0), first["provider_write_call_count"])
	assert.Equal(t, "metadata_only", first["retention_class"])
}
