package readiness_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

func TestProjection_ValidatesIndexesAndProducesPortableReadiness(t *testing.T) {
	t.Parallel()

	result, err := readiness.Project(context.Background(), portableInput(t, fixtureRoot(t)))
	if err != nil {
		t.Fatalf("Project() returned error for valid non-Autopus fixture: %v", err)
	}
	if result.Projection == nil {
		t.Fatal("Project() did not produce a readiness projection")
	}

	projection := result.Projection
	if projection.ReleaseVerdict != readiness.ReleaseVerdictBlocked {
		t.Fatalf("release verdict = %q, want %q", projection.ReleaseVerdict, readiness.ReleaseVerdictBlocked)
	}
	wantStatuses := map[string]readiness.Status{
		"fast":             readiness.StatusPassed,
		"browser-staging":  readiness.StatusFailed,
		"desktop-native":   readiness.StatusBlocked,
		"gui-explore":      readiness.StatusSkipped,
		"mobile-readiness": readiness.StatusDeferred,
		"canary-explicit":  readiness.StatusSetupGap,
	}
	if !reflect.DeepEqual(projection.LaneStatuses, wantStatuses) {
		t.Fatalf("lane statuses = %#v, want %#v", projection.LaneStatuses, wantStatuses)
	}
	if projection.CheckCounts.Total != 3 || projection.CheckCounts.Failed != 1 || projection.CheckCounts.Passed != 1 || projection.CheckCounts.Skipped != 1 {
		t.Fatalf("check counts = %#v, want total=3 passed=1 failed=1 skipped=1", projection.CheckCounts)
	}
	if len(projection.SetupGaps) != 1 || projection.SetupGaps[0].Class != "env-missing" {
		t.Fatalf("setup gaps = %#v, want one env-missing gap", projection.SetupGaps)
	}
	if !reflect.DeepEqual(projection.DeferredLanes, []string{"mobile-readiness"}) {
		t.Fatalf("deferred lanes = %#v, want mobile-readiness", projection.DeferredLanes)
	}
	if len(projection.EvidenceRefs) != 1 || projection.EvidenceRefs[0].ManifestPath != "qa/evidence/manifests/login.json" {
		t.Fatalf("evidence refs = %#v, want redacted publishable manifest ref", projection.EvidenceRefs)
	}
	if len(projection.FeedbackRefs) != 1 || projection.FeedbackRefs[0].BundlePath != "qa/feedback/login-codex/bundle.json" {
		t.Fatalf("feedback refs = %#v, want portable feedback bundle ref", projection.FeedbackRefs)
	}
	assertSourceChain(t, projection)
	if projection.LastRunTime != "2026-05-15T10:01:00Z" {
		t.Fatalf("last run time = %q, want fixture ended_at", projection.LastRunTime)
	}
	if projection.TrendSummary != "1 failed deterministic check across 6 lanes" {
		t.Fatalf("trend summary = %q", projection.TrendSummary)
	}
	if len(projection.FeedbackActions) != 1 || !projection.FeedbackActions[0].Enabled {
		t.Fatalf("feedback actions = %#v, want enabled repair handoff for failed deterministic redacted evidence", projection.FeedbackActions)
	}

	body := mustJSON(t, projection)
	for _, forbidden := range []string{"Autopus/", "autopus-desktop", "workspace route", "desktop IPC"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("portable projection leaked product-specific contract %q in %s", forbidden, body)
		}
	}
}

func TestProjection_DisablesFeedbackActionForIneligibleEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(t *testing.T, root string)
		wantReason string
	}{
		{
			name: "passed evidence",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "evidence", "manifests", "login.json"), func(doc map[string]any) {
					doc["status"] = "passed"
				})
			},
			wantReason: "not_failed",
		},
		{
			name: "non deterministic release row",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
					rows := doc["lane_rows"].([]any)
					row := rows[1].(map[string]any)
					row["deterministic_authority"] = false
				})
			},
			wantReason: "not_deterministic",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := copyFixture(t)
			tt.mutate(t, root)

			result, err := readiness.Project(context.Background(), portableInput(t, root))
			if err != nil {
				t.Fatalf("Project() returned error: %v", err)
			}
			action := result.Projection.FeedbackActions[0]
			if action.Enabled || action.DisabledReason != tt.wantReason || len(action.Command) != 0 || action.CommandDisplay != "" {
				t.Fatalf("feedback action = %#v, want disabled reason %q without command", action, tt.wantReason)
			}
		})
	}
}

func TestProjection_RejectsInvalidSchemaRefsRedactionPublishabilityAndOwnership(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(t *testing.T, root string)
		wantClass string
	}{
		{
			name: "unsupported run schema",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "run-index.json"), func(doc map[string]any) {
					doc["schema_version"] = "qamesh.run_index.v0"
				})
			},
			wantClass: "invalid_schema:run_index",
		},
		{
			name: "missing source refs",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
					delete(doc, "source_refs")
				})
			},
			wantClass: "invalid_ref:missing_source_ref",
		},
		{
			name: "redaction failed",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "run-index.json"), func(doc map[string]any) {
					doc["redaction_status"] = map[string]any{"status": "failed"}
				})
			},
			wantClass: "unsafe_redaction:failed",
		},
		{
			name: "unpublishable artifact ref",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "evidence", "manifests", "login.json"), func(doc map[string]any) {
					artifacts := doc["artifacts"].([]any)
					artifact := artifacts[0].(map[string]any)
					artifact["publishable"] = false
					artifact["path"] = "qa/evidence/_raw/login.png"
				})
			},
			wantClass: "unsafe_ref:unpublishable_artifact_or_media",
		},
		{
			name: "unsupported manifest schema",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "evidence", "manifests", "login.json"), func(doc map[string]any) {
					doc["schema_version"] = "qamesh.evidence.v0"
				})
			},
			wantClass: "invalid_schema:evidence_manifest",
		},
		{
			name: "manifest redaction failed",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "evidence", "manifests", "login.json"), func(doc map[string]any) {
					doc["redaction_status"] = map[string]any{"status": "failed"}
				})
			},
			wantClass: "unsafe_redaction:manifest_failed",
		},
		{
			name: "workspace repo mismatch",
			mutate: func(t *testing.T, root string) {
				patchJSON(t, filepath.Join(root, "qa", "release-index.json"), func(doc map[string]any) {
					doc["workspace"] = map[string]any{
						"workspace_id": "other-workspace",
						"repo_id":      "other-repo",
						"repo_root":    "repos/other-repo",
					}
				})
			},
			wantClass: "invalid_owner:workspace_repo_mismatch",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := copyFixture(t)
			tt.mutate(t, root)

			result, err := readiness.Project(context.Background(), portableInput(t, root))
			if err == nil {
				t.Fatalf("Project() unexpectedly accepted %s", tt.name)
			}
			assertFailClosed(t, result, []string{tt.wantClass}, []string{"sk-prod", "/Users/alice", "Cookie: session", "_raw/login.png"})
		})
	}
}

func assertSourceChain(t *testing.T, projection *readiness.Projection) {
	t.Helper()
	for _, chain := range projection.SourceChains {
		if chain.Lane != "browser-staging" {
			continue
		}
		if chain.ReleaseIndexPath != "qa/release-index.json" ||
			chain.RunIndexPath != "qa/run-index.json" ||
			chain.ManifestPath != "qa/evidence/manifests/login.json" ||
			chain.FeedbackBundle != "qa/feedback/login-codex/bundle.json" ||
			chain.AuditRef != "qamesh-audit:portable-shop:rel-portable-001" ||
			len(chain.SourceRefs) != 1 {
			t.Fatalf("browser source chain = %#v, want release/run/manifest/feedback/audit/source refs", chain)
		}
		return
	}
	t.Fatalf("source chains = %#v, want browser-staging evidence chain", projection.SourceChains)
}
