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

// TestQAFullCmd_DefaultPlansFullGateWithoutSideEffects also pins D16: a blocked
// plan is an error envelope with a non-zero exit, never envelope warn over
// data.summary.status "blocked". `qa release` maps blocked the same way.
func TestQAFullCmd_DefaultPlansFullGateWithoutSideEffects(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"test":"node test.js"}}`), 0o644))
	output := filepath.Join(dir, ".autopus", "qa", "releases")

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"full", "--project-dir", dir, "--output", output, "--format", "json"})

	require.Error(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assertCommonJSONEnvelope(t, payload, "qa full")
	assert.Equal(t, "error", payload["status"])
	assert.Equal(t, "qa_full_blocked", payload["error"].(map[string]any)["code"])
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

	// The plan is legitimately blocked here: `qa init` deliberately ships no
	// gui-explore Journey Pack, and gui-explore is a `must` lane once browser
	// signals are present, so `--run` on this project returns blocked too. The
	// error envelope still carries the full payload.
	require.Error(t, cmd.Execute())
	payload := decodeJSONMap(t, out.Bytes())
	assert.Equal(t, "error", payload["status"])
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

// TestQAFullTextModePrintsRootBlockerOnFailure is the D15 guard: the failure path
// used to return a bare "qa release blocked" and discard the root blocker lane,
// reason, failed journey, and failure summary the payload already carried.
func TestQAFullTextModePrintsRootBlockerOnFailure(t *testing.T) {
	t.Parallel()

	cmd := newQAFullCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)

	payload := buildQAFullRunPayload(
		qaFullOptions{ProjectDir: "."},
		qarelease.ExecutionPayload{Index: qarelease.Index{
			Profile:       "prelaunch",
			Status:        qarelease.GateStatusBlocked,
			SelectedLanes: []string{"fast"},
			LaneRows: []qarelease.LaneRow{{
				Lane:            "fast",
				Status:          qarelease.LaneStatusFailed,
				LaneVerdict:     qarelease.LaneVerdictBlock,
				FailedJourneyID: "go-fast",
				FailureSummary:  "expected exit_code=0, got 1",
				Blockers:        []qarelease.Blocker{{Lane: "fast", Reason: "journey_failed:go-fast"}},
			}},
		}},
		qaFullDomainReadiness{Status: "ready"},
		nil,
	)
	writeQAFullText(cmd, payload)

	text := out.String()
	assert.Contains(t, text, "lane=fast")
	assert.Contains(t, text, "reason=journey_failed:go-fast")
	assert.Contains(t, text, "journey=go-fast")
	assert.Contains(t, text, "summary=expected exit_code=0, got 1")
	assert.Contains(t, text, "next: auto qa feedback")
}

// TestQAFullRunSummaryCountsJourneyPacksNotLanes is the D17 guard: run mode used
// to report len(LaneRows) under journey_pack_count, so a two-pack project claimed
// seven. Both modes must count Journey Packs.
func TestQAFullRunSummaryCountsJourneyPacksNotLanes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packs := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(packs, 0o755))
	for _, id := range []string{"go-fast", "canary-explicit"} {
		body := "id: " + id + "\ntitle: " + id + "\nsurface: cli\nlanes: [fast]\n" +
			"adapter:\n  id: go-test\ncommand:\n  argv: [\"go\", \"test\", \"./...\"]\n  cwd: .\n  timeout: 60s\n" +
			"checks:\n  - id: unit\n    type: unit_test\n"
		require.NoError(t, os.WriteFile(filepath.Join(packs, id+".yaml"), []byte(body), 0o644))
	}

	lanes := qarelease.ReleaseLanes()
	rows := make([]qarelease.LaneRow, 0, len(lanes))
	for _, lane := range lanes {
		rows = append(rows, qarelease.LaneRow{Lane: lane, LaneVerdict: qarelease.LaneVerdictPass})
	}

	payload := buildQAFullRunPayload(
		qaFullOptions{ProjectDir: dir},
		qarelease.ExecutionPayload{Index: qarelease.Index{SelectedLanes: lanes, LaneRows: rows}},
		qaFullDomainReadiness{Status: "ready"},
		nil,
	)

	assert.Len(t, payload.Summary.SelectedLanes, len(lanes))
	assert.Equal(t, 2, payload.Summary.JourneyPackCount, "journey_pack_count must count packs, not lanes")
}
