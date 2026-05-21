package memindex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/learn"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestMemIndexRebuildSearchStatusContextEndToEnd(t *testing.T) {
	t.Parallel()

	projectDir := makeMemIndexFixture(t)
	result, err := Rebuild(Options{ProjectDir: projectDir, IndexPath: "mem-test.sqlite"})
	require.NoError(t, err)

	assert.Equal(t, SchemaVersion, result.SchemaVersion)
	assert.True(t, result.ProjectionOnly)
	assert.True(t, result.FTS5Verified)
	assert.Equal(t, "ok", result.FTS5Probe.Status)
	assert.Contains(t, result.IndexPath, filepath.Join(".autopus", "runtime", "memindex", "mem-test.sqlite"))
	assert.GreaterOrEqual(t, result.CountsBySourceKind["project_doc"], 1)
	assert.GreaterOrEqual(t, result.CountsBySourceKind["review_failure"], 1)
	assert.GreaterOrEqual(t, result.CountsBySourceKind["spec"], 1)
	assert.GreaterOrEqual(t, result.CountsBySourceKind["learning"], 1)
	assert.GreaterOrEqual(t, result.CountsBySourceKind["qamesh_failed_check"], 1)
	assert.GreaterOrEqual(t, result.CountsBySourceKind["qamesh_setup_gap"], 1)
	assert.GreaterOrEqual(t, result.CountsBySourceKind["qamesh_repair_prompt"], 1)
	assert.Equal(t, 1, result.SkippedCountsByReason["invalid_learning_jsonl"])
	assert.Equal(t, 1, result.SkippedCountsByReason["invalid_qamesh_run_index"])
	assert.Equal(t, 1, result.SkippedCountsByReason["unredacted_qamesh_run"])
	assert.GreaterOrEqual(t, result.SkippedCountsByReason[SkipReasonWorkspaceGeneratedSurface], 1)
	assert.GreaterOrEqual(t, result.SkippedCountsByReason[SkipReasonWorkspaceHarnessSurface], 1)
	assert.NotEmpty(t, result.SourceHashes["L-001"])
	assert.NotEmpty(t, result.IndexedSources)

	search, err := Search(SearchOptions{
		ProjectDir:   projectDir,
		IndexPath:    "mem-test.sqlite",
		Query:        "approval drift stable role selectors",
		TopK:         10,
		RequireFresh: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, search.Results)
	assert.Equal(t, SchemaVersion, search.SchemaVersion)
	assert.Equal(t, 10, search.TopK)
	for i, got := range search.Results {
		assert.Equal(t, i+1, got.Rank)
		assert.Equal(t, Fresh, got.FreshnessState)
		assert.Equal(t, Redacted, got.RedactionStatus)
		assert.NotEmpty(t, got.SnippetDigest)
		assert.LessOrEqual(t, len(got.SnippetDigest), 160)
	}
	assert.Contains(t, sourceTypes(search.Results), "learning")

	repairSearch, err := Search(SearchOptions{
		ProjectDir:   projectDir,
		IndexPath:    "mem-test.sqlite",
		Query:        "repair prompt",
		TopK:         10,
		RequireFresh: true,
	})
	require.NoError(t, err)
	assert.Contains(t, sourceTypes(repairSearch.Results), "qamesh_repair_prompt")

	context, err := Context(ContextOptions{
		ProjectDir:   projectDir,
		IndexPath:    "mem-test.sqlite",
		Query:        "approval drift stable role selectors deterministic QA failure",
		TopK:         20,
		BudgetTokens: 260,
	})
	require.NoError(t, err)
	assert.Contains(t, context.Prompt, "## Quality Recall")
	assert.Contains(t, context.Prompt, "source_ref:")
	assert.Contains(t, context.Prompt, "next_action:")
	assert.Greater(t, context.OmittedCount, 0)
	assert.NotEmpty(t, context.Results)

	status, err := Status(Options{ProjectDir: projectDir, IndexPath: "mem-test.sqlite"})
	require.NoError(t, err)
	assert.False(t, status.CorruptState.IsCorrupt)
	assert.False(t, status.RebuildRecommended)
	assert.Empty(t, status.StaleRefs)
	assert.Equal(t, result.CountsBySourceKind["learning"], status.CountsBySourceKind["learning"])

	specPath := filepath.Join(projectDir, ".autopus", "specs", "SPEC-AUTO-MEM-001", "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# SPEC-AUTO-MEM-001\n\napproval drift changed after indexing\n"), 0o644))

	staleStatus, err := Status(Options{ProjectDir: projectDir, IndexPath: "mem-test.sqlite"})
	require.NoError(t, err)
	assert.True(t, staleStatus.RebuildRecommended)
	assert.Contains(t, staleStatus.StaleRefs, ".autopus/specs/SPEC-AUTO-MEM-001/spec.md")

	_, err = Search(SearchOptions{
		ProjectDir:   projectDir,
		IndexPath:    "mem-test.sqlite",
		Query:        "approval drift",
		RequireFresh: true,
	})
	require.Error(t, err)
	var memErr *Error
	require.ErrorAs(t, err, &memErr)
	assert.Equal(t, "stale-source", memErr.Code)
}

func makeMemIndexFixture(t *testing.T) string {
	t.Helper()

	projectDir := t.TempDir()
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "project", "overview.md"), `# Project Overview

Memory index recall keeps approval drift and deterministic QA evidence searchable.
`)
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "project", "reviews", "approval-review.md"), `# Review Failure

approval drift appeared during review and needs stable recall.
`)
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "specs", "SPEC-AUTO-MEM-001", "spec.md"), `# SPEC-AUTO-MEM-001

The system shall recall approval drift decisions.

AC-AUTO-MEM-001 confirms indexed context includes source references.
`)
	writeMemIndexFile(t, filepath.Join(projectDir, ".codex", "rules", "generated.md"), "# generated\n")
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "plugins", "generated.md"), "# generated\n")
	writeMemIndexLearning(t, projectDir)
	writeMemIndexQAMESH(t, projectDir)
	return projectDir
}

func writeMemIndexLearning(t *testing.T, projectDir string) {
	t.Helper()

	entry := learn.LearningEntry{
		ID:         "L-001",
		Timestamp:  time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC),
		Type:       learn.EntryTypeFixPattern,
		Phase:      "review",
		SpecID:     "SPEC-AUTO-MEM-001",
		Files:      []string{"pkg/memindex/query.go"},
		Packages:   []string{"pkg/memindex"},
		Pattern:    "approval drift stable role selectors",
		Resolution: "reuse memory search context before changing acceptance criteria",
		Severity:   learn.SeverityHigh,
		ReuseCount: 2,
	}
	body, err := json.Marshal(entry)
	require.NoError(t, err)
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "learnings", "pipeline.jsonl"), string(body)+"\n{invalid-json}\n")
}

func writeMemIndexQAMESH(t *testing.T, projectDir string) {
	t.Helper()

	runDir := filepath.Join(projectDir, ".autopus", "qa", "runs", "qa-mem-001")
	writeMemIndexJSON(t, filepath.Join(runDir, "run-index.json"), qarun.Index{
		SchemaVersion: qarun.RunIndexSchemaVersion,
		RunID:         "qa-mem-001",
		Status:        "failed",
		StartedAt:     "2026-05-06T00:00:00Z",
		EndedAt:       "2026-05-06T00:00:01Z",
		Profile:       "default",
		Lane:          "golden",
		ManifestPaths: []string{"manifest.json"},
		Checks: []qarun.IndexCheck{{
			ID:             "login-submit-check",
			JourneyID:      "login",
			Adapter:        "playwright",
			Status:         "failed",
			Expected:       "button visible",
			Actual:         "button missing",
			FailureSummary: "deterministic QA failure needs stable role selectors",
		}},
		AdapterResults: []qarun.AdapterResult{{
			Adapter:               "playwright",
			JourneyID:             "login",
			Status:                "failed",
			QAMESHManifestPath:    "manifest.json",
			RepairPromptAvailable: true,
			FailureSummary:        "AC-AUTO-MEM-001 deterministic QA failure",
		}},
		SetupGaps: []qarun.SetupGap{{
			Adapter:   "playwright",
			JourneyID: "login",
			Reason:    "browser dependency missing",
		}},
		FeedbackBundlePaths: []string{"feedback/codex"},
		RedactionStatus:     qarun.RedactionStatus{Status: "passed"},
	})
	writeMemIndexJSON(t, filepath.Join(runDir, "manifest.json"), qaevidence.Manifest{
		SchemaVersion: qaevidence.SchemaVersion,
		QAResultID:    "qa-login-001",
		Surface:       "browser",
		Lane:          "golden",
		ScenarioRef:   "login",
		Runner:        qaevidence.Runner{Name: "playwright"},
		Status:        "failed",
		StartedAt:     "2026-05-06T00:00:00Z",
		EndedAt:       "2026-05-06T00:00:01Z",
		OracleResults: qaevidence.OracleResults{Checks: []qaevidence.CheckResult{{
			ID:             "login-submit-check",
			Type:           "exit_code",
			Status:         "failed",
			Expected:       "0",
			Actual:         "1",
			FailureSummary: "deterministic QA failure needs stable role selectors",
		}}},
		RedactionStatus: qaevidence.RedactionStatus{Status: "passed"},
		SourceRefs: qaevidence.SourceRefs{
			SourceSpec:     "SPEC-AUTO-MEM-001",
			AcceptanceRefs: []string{"AC-AUTO-MEM-001"},
			OwnedPaths:     []string{"e2e/login.spec.ts"},
		},
		RepairPromptRef: "feedback/codex/repair-prompt.md",
		RetentionClass:  "local-redacted",
	})
	writeMemIndexFile(t, filepath.Join(projectDir, ".autopus", "qa", "runs", "bad-json", "run-index.json"), "{")
	writeMemIndexJSON(t, filepath.Join(projectDir, ".autopus", "qa", "runs", "unredacted", "run-index.json"), qarun.Index{
		SchemaVersion:   qarun.RunIndexSchemaVersion,
		RunID:           "qa-unredacted",
		Status:          "failed",
		RedactionStatus: qarun.RedactionStatus{Status: "failed"},
	})
}

func writeMemIndexFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func writeMemIndexJSON(t *testing.T, path string, value any) {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	require.NoError(t, err)
	writeMemIndexFile(t, path, string(body)+"\n")
}

func sourceTypes(results []SearchResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.SourceType)
	}
	return out
}
