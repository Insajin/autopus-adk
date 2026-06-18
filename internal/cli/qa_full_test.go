package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

func TestQAFullCmd_DefaultPlansFullGateWithoutSideEffects(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js"}}`), 0o644))
	output := filepath.Join(dir, ".autopus", "qa", "releases")

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"full", "--project-dir", dir, "--output", output, "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa full")
	assert.Equal(t, "warn", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, "qamesh.full.v1", data["schema_version"])
	assert.Equal(t, "plan", data["mode"])
	assert.NoDirExists(t, output)

	summary := data["summary"].(map[string]any)
	assert.Equal(t, "blocked", summary["status"])
	assert.NotZero(t, int(summary["setup_gap_count"].(float64)))
	assert.Contains(t, stringSlice(summary["selected_lanes"]), "fast")

	policy := data["qa_policy"].(map[string]any)
	assert.Equal(t, "qamesh", policy["orchestrator"])
	assert.Equal(t, "auto-detected-from-project-journey-packs", policy["runner_selection"])
	assert.Contains(t, stringSlice(policy["user_choice_required_for"]), "execution")

	releasePlan := data["release_plan"].(map[string]any)
	assert.Equal(t, "qamesh.release_plan.v1", releasePlan["schema_version"])

	domain := data["domain_readiness"].(map[string]any)
	assert.Equal(t, "setup_gap", domain["status"])
	assert.Contains(t, domain["setup_gap"], "domain readiness catalog is missing")

	next := stringSlice(data["next_commands"])
	assert.Contains(t, next[0], "auto qa full --run")
	assert.Contains(t, next, "auto qa domain-readiness init --project-dir "+shellWord(dir)+" --format json")
}

func TestQAFullCmd_SelectsProjectWhenWorkspaceRootHasMultipleTargets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, name := range []string{"frontend", "desktop"} {
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js"}}`), 0o644))
	}

	cmd := newQAFullCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, runQAFull(cmd, qaFullOptions{ProjectDir: root, Format: "json"}))
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "warn", payload["status"])
	data := payload["data"].(map[string]any)
	assert.Equal(t, "select_project", data["mode"])
	policy := data["qa_policy"].(map[string]any)
	assert.Equal(t, "qamesh", policy["orchestrator"])
	assert.Contains(t, stringSlice(policy["runner_adapters"]), "node-script")
	assert.Contains(t, stringSlice(policy["user_choice_required_for"]), "project-dir")
	candidates := data["project_candidates"].([]any)
	assert.Len(t, candidates, 2)
	next := stringSlice(data["next_commands"])
	assert.Contains(t, next, "auto qa full --bootstrap --project-dir desktop --format json")
	assert.Contains(t, next, "auto qa full --project-dir frontend --format json")
}

func TestQAFullCmd_BootstrapCreatesSafeStarterFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js","build":"vite build"},"dependencies":{"vite":"^7.0.0"}}`), 0o644))

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"full", "--project-dir", dir, "--bootstrap", "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	data := payload["data"].(map[string]any)
	bootstrap := data["bootstrap"].(map[string]any)
	assert.Equal(t, "created", bootstrap["status"])
	policy := data["qa_policy"].(map[string]any)
	assert.Equal(t, "qamesh", policy["orchestrator"])
	assert.Contains(t, stringSlice(policy["runner_adapters"]), "node-script")
	assert.Contains(t, stringSlice(policy["runner_adapters"]), "playwright")
	assert.Contains(t, policy["playwright_role"], "not a competing QA mode")
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "journeys", "node-fast.yaml"))
	assert.FileExists(t, filepath.Join(dir, ".autopus", "qa", "domain-readiness", "catalog.json"))
}

func TestQAFullCmd_IsRegisteredUnderQA(t *testing.T) {
	t.Parallel()

	root := NewRootCmd()
	fullCmd, _, err := root.Find([]string{"qa", "full"})
	require.NoError(t, err)
	require.NotNil(t, fullCmd)
}

func TestBuildQAFullRunPayloadIncludesRootJourneyBlocker(t *testing.T) {
	t.Parallel()

	payload := buildQAFullRunPayload(
		qaFullOptions{ProjectDir: "desktop"},
		qarelease.ExecutionPayload{Index: qarelease.Index{
			Profile:       "prelaunch",
			Status:        qarelease.GateStatusBlocked,
			SelectedLanes: []string{"fast"},
			LaneRows: []qarelease.LaneRow{{
				Lane:            "fast",
				Status:          qarelease.LaneStatusFailed,
				LaneVerdict:     qarelease.LaneVerdictBlock,
				FailedJourneyID: "desktop-messenger-core",
				FailureSummary:  "expected exit_code=0",
				Blockers:        []qarelease.Blocker{{Lane: "fast", Reason: "journey_failed:desktop-messenger-core"}},
				ManifestPaths:   []string{".autopus/qa/runs/r1/desktop-messenger-core/manifest.json"},
				FeedbackRefs:    []string{},
				SetupGapClass:   qarelease.SetupGapNone,
				Severity:        qarelease.SeverityHigh,
				LanePolicy:      qarelease.LanePolicyMust,
				RunIndexPath:    ".autopus/qa/runs/r1/run-index.json",
				OwnerSpec:       "SPEC-QAMESH-002",
				OwnerRepo:       "autopus-desktop",
				SkippedReason:   "",
			}},
		}},
		qaFullDomainReadiness{Status: "ready"},
		nil,
	)

	assert.Equal(t, "blocked", payload.Summary.Status)
	assert.Equal(t, "qamesh", payload.QAPolicy.Orchestrator)
	assert.Contains(t, payload.QAPolicy.UserChoiceRequiredFor, "execution")
	assert.Equal(t, "fast", payload.Summary.RootBlockerLane)
	assert.Equal(t, "journey_failed:desktop-messenger-core", payload.Summary.RootBlockerReason)
	assert.Equal(t, "desktop-messenger-core", payload.Summary.RootFailedJourneyID)
	assert.Equal(t, "expected exit_code=0", payload.Summary.RootFailureSummary)
}
