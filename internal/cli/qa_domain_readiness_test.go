package cli

import (
	"bytes"
	"os"
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

// TestQADomainReadinessInitIsIdempotent is the D24 guard: init used to create the
// catalog then exit 1 with "catalog already exists" on every later run, which
// broke idempotent provisioning scripts. Sibling `qa full --bootstrap` reports
// created/skipped and exits 0; this now matches.
func TestQADomainReadinessInitIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	run := func() map[string]any {
		cmd := newQACmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"domain-readiness", "init", "--project-dir", dir, "--format", "json"})
		require.NoError(t, cmd.Execute())
		payload := decodeJSONMap(t, out.Bytes())
		assert.Equal(t, "ok", payload["status"])
		return payload["data"].(map[string]any)
	}

	first := run()
	assert.Equal(t, "created", first["status"])
	assert.Equal(t, true, first["created"])

	second := run()
	assert.Equal(t, "skipped", second["status"])
	assert.Equal(t, false, second["created"])
	assert.NotEmpty(t, second["reason"])
	assert.Equal(t, first["catalog_path"], second["catalog_path"])
}

// TestQADomainReadinessPlanReportsDanglingJourneyRefs is the D20 CLI guard: a
// catalog whose journey_pack_refs name packs the project does not own must not
// print valid=true, and the envelope must not say ok while the data says invalid.
func TestQADomainReadinessPlanReportsDanglingJourneyRefs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	catalogPath := filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(catalogPath), 0o755))
	require.NoError(t, os.WriteFile(catalogPath, []byte(danglingRefCatalogJSON), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"domain-readiness", "plan", "--project-dir", dir, "--lane", "fast", "--format", "json"})
	require.NoError(t, cmd.Execute())

	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "warn", payload["status"])
	plan := payload["data"].(map[string]any)
	assert.Equal(t, false, plan["valid"])
	gaps := plan["journey_ref_gaps"].([]any)
	require.Len(t, gaps, 1)
	gap := gaps[0].(map[string]any)
	assert.Equal(t, "node-build-fast", gap["journey_pack_ref"])
	assert.Equal(t, "journey_pack_not_found", gap["reason"])

	textCmd := newQACmd()
	var textOut bytes.Buffer
	textCmd.SetOut(&textOut)
	textCmd.SetArgs([]string{"domain-readiness", "plan", "--project-dir", dir, "--lane", "fast"})
	require.NoError(t, textCmd.Execute())
	assert.Contains(t, textOut.String(), "valid=false")
	assert.Contains(t, textOut.String(), "journey_ref_gaps=1")
	assert.Contains(t, textOut.String(), "journey_pack_ref=node-build-fast")
}

const danglingRefCatalogJSON = `{
  "schema_version": "qamesh.domain_readiness_catalog.v1",
  "suite_id": "project-domain-readiness",
  "required_domains": ["core"],
  "scenarios": [
    {
      "schema_version": "qamesh.domain_readiness_scenario.v1",
      "scenario_id": "project-core-readiness",
      "domain": "core",
      "owner": "project-owner",
      "owning_repo": ".",
      "source_spec_refs": ["SPEC-QAMESH-002"],
      "scenario_mode": "contract_test",
      "mutation_boundary": "read_only",
      "fixture_or_source_need": ["project-owned deterministic test evidence"],
      "journey_pack_refs": ["node-build-fast"],
      "qamesh_lane_refs": ["fast"],
      "expected_evidence": ["deterministic_check_result"],
      "pass_fail_oracle": ["exit_code == 0"],
      "freshness_window_hours": 24,
      "forbidden_actions": ["production_mutation", "provider_write"],
      "launch_quality_domain": "core",
      "safe_execution_environment": {
        "kind": "local_safe_shell",
        "environment": "local",
        "cwd": ".",
        "timeout": "5m"
      }
    }
  ]
}
`
