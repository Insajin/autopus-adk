package promptlayer_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestOMPContextEvidenceStoreSecurity_StrictJSONRejectsDuplicateAndTrailingValues(t *testing.T) {
	tests := []struct {
		name, needle, replacement string
	}{
		{"top level", `"schema_version":"autopus.omp_context_evidence_store.v1"`, `"schema_version":"autopus.omp_context_evidence_store.v1","schema_version":"autopus.omp_context_evidence_store.v1"`},
		{"binding", `"workspace_id":"workspace-1"`, `"workspace_id":"workspace-1","workspace_id":"workspace-1"`},
		{"policy", `"profile":"history-v1"`, `"profile":"history-v1","profile":"history-v1"`},
		{"canary row", `"tokens":10000`, `"tokens":10000,"tokens":10000`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newEvidenceSecurityFixture(t)
			body, err := os.ReadFile(fixture.path)
			require.NoError(t, err)
			require.Contains(t, string(body), test.needle)
			tampered := strings.Replace(string(body), test.needle, test.replacement, 1)
			require.NoError(t, os.WriteFile(fixture.path, []byte(tampered), 0o600))
			_, err = fixture.load(fixture.checkedAt.Add(time.Minute), fixture.expectation, fixture.policy)
			require.ErrorContains(t, err, "duplicate key")
		})
	}

	t.Run("trailing JSON", func(t *testing.T) {
		fixture := newEvidenceSecurityFixture(t)
		file, err := os.OpenFile(fixture.path, os.O_APPEND|os.O_WRONLY, 0)
		require.NoError(t, err)
		_, err = file.WriteString("{}\n")
		require.NoError(t, err)
		require.NoError(t, file.Close())
		_, err = fixture.load(fixture.checkedAt.Add(time.Minute), fixture.expectation, fixture.policy)
		require.ErrorContains(t, err, "trailing JSON")
	})
}

func TestOMPContextEvidenceStoreSecurity_RejectsUnsafeModesAndIntermediateSymlink(t *testing.T) {
	tests := []struct {
		name string
		path func(evidenceSecurityFixture) string
		mode os.FileMode
	}{
		{"file mode", func(f evidenceSecurityFixture) string { return f.path }, 0o644},
		{"directory mode", func(f evidenceSecurityFixture) string { return filepath.Dir(f.path) }, 0o755},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newEvidenceSecurityFixture(t)
			require.NoError(t, os.Chmod(test.path(fixture), test.mode))
			_, err := fixture.load(fixture.checkedAt.Add(time.Minute), fixture.expectation, fixture.policy)
			require.ErrorContains(t, err, "unsafe")
		})
	}

	t.Run("intermediate symlink", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, ".autopus"), 0o700))
		require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(root, ".autopus", "runtime")))
		document := evidenceSecurityDocument(time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC))
		require.ErrorContains(t, promptlayer.WriteOMPContextEvidenceStoreV1(root, document), "unsafe")
	})
}

func TestOMPContextEvidenceStoreSecurity_FreshnessBoundariesFailClosed(t *testing.T) {
	fixture := newEvidenceSecurityFixture(t)
	tests := []struct {
		name    string
		now     time.Time
		wantErr bool
	}{
		{"future skew boundary", fixture.checkedAt.Add(-5 * time.Minute), false},
		{"before future skew", fixture.checkedAt.Add(-5*time.Minute - time.Nanosecond), true},
		{"before expiry", fixture.checkedAt.Add(24*time.Hour - time.Nanosecond), false},
		{"at expiry", fixture.checkedAt.Add(24 * time.Hour), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verified, err := fixture.load(test.now, fixture.expectation, fixture.policy)
			if test.wantErr {
				require.ErrorContains(t, err, "stale or future-dated")
				return
			}
			require.NoError(t, err)
			require.Len(t, verified.Promotion.Rows, 40)
		})
	}
}

func TestOMPContextEvidenceStoreSecurity_RejectsPolicyBindingAndRuntimeMismatch(t *testing.T) {
	tests := []struct {
		name, want string
		mutate     func(*testing.T, evidenceSecurityFixture, *promptlayer.OMPContextEvidenceExpectationV1, *promptlayer.OMPContextPromotionPolicyV1)
	}{
		{"stored policy digest", "policy mismatch", func(t *testing.T, f evidenceSecurityFixture, _ *promptlayer.OMPContextEvidenceExpectationV1, _ *promptlayer.OMPContextPromotionPolicyV1) {
			var document promptlayer.OMPContextEvidenceStoreV1
			body, err := os.ReadFile(f.path)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &document))
			document.Binding.PolicyDigest = "sha256:" + strings.Repeat("f", 64)
			body, err = json.Marshal(document)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(f.path, append(body, '\n'), 0o600))
		}},
		{"expected policy", "policy mismatch", func(_ *testing.T, _ evidenceSecurityFixture, _ *promptlayer.OMPContextEvidenceExpectationV1, p *promptlayer.OMPContextPromotionPolicyV1) {
			p.HistoryTargetTokens++
		}},
		{"snapshot binding", "binding mismatch", func(_ *testing.T, _ evidenceSecurityFixture, e *promptlayer.OMPContextEvidenceExpectationV1, _ *promptlayer.OMPContextPromotionPolicyV1) {
			e.SnapshotHash = "sha256:" + strings.Repeat("f", 64)
		}},
		{"runtime binding", "binding mismatch", func(_ *testing.T, _ evidenceSecurityFixture, e *promptlayer.OMPContextEvidenceExpectationV1, _ *promptlayer.OMPContextPromotionPolicyV1) {
			e.RuntimeVersion = "omp/17.1.9"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newEvidenceSecurityFixture(t)
			expectation, policy := fixture.expectation, fixture.policy
			test.mutate(t, fixture, &expectation, &policy)
			_, err := fixture.load(fixture.checkedAt.Add(time.Minute), expectation, policy)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestOMPContextEvidenceStoreSecurity_RejectsInvalidPromotionAndDuplicateHistory(t *testing.T) {
	tests := []struct {
		name, want string
		mutate     func(*promptlayer.OMPContextEvidenceStoreV1)
	}{
		{"security-failed canary", "canary evidence rejected", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.CanaryRows[1].SecurityPassed = false }},
		{"duplicate history ID", "duplicate", func(d *promptlayer.OMPContextEvidenceStoreV1) {
			d.HistoryRefs = append(d.HistoryRefs, d.HistoryRefs[0])
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := evidenceSecurityDocument(time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC))
			test.mutate(&document)
			require.ErrorContains(t, promptlayer.WriteOMPContextEvidenceStoreV1(t.TempDir(), document), test.want)
		})
	}
}

func TestOMPContextEvidenceStoreSecurity_RejectsEveryAuthorityEnvelopeDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*promptlayer.OMPContextEvidenceStoreV1)
	}{
		{"schema", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.SchemaVersion = "v2" }},
		{"workspace metadata", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Binding.WorkspaceID = "bad\nworkspace" }},
		{"snapshot hash", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Binding.SnapshotHash = "bad" }},
		{"short git", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Binding.GitCommitHash = "abc" }},
		{"non-hex git", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Binding.GitCommitHash = strings.Repeat("g", 40) }},
		{"validity", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Binding.ValidFor = 0 }},
		{"inactive policy", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Policy.HistoryMode = "shadow" }},
		{"oversized target", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.Policy.HistoryTargetTokens = 1_000_000_001 }},
		{"missing canary", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.CanaryRows = nil }},
		{"incomplete canary", func(d *promptlayer.OMPContextEvidenceStoreV1) { d.CanaryRows = d.CanaryRows[:1] }},
		{"too many history refs", func(d *promptlayer.OMPContextEvidenceStoreV1) {
			d.HistoryRefs = make([]promptlayer.OMPContextHistoryReference, 4097)
		}},
		{"invalid history token count", func(d *promptlayer.OMPContextEvidenceStoreV1) {
			d.HistoryRefs[0].TokenEstimate = 0
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := evidenceSecurityDocument(time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC))
			test.mutate(&document)
			require.Error(t, promptlayer.WriteOMPContextEvidenceStoreV1(t.TempDir(), document))
		})
	}
}

func TestOMPContextEvidenceStoreSecurity_RejectsOversizedArtifacts(t *testing.T) {
	document := evidenceSecurityDocument(time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC))
	document.HistoryRefs = make([]promptlayer.OMPContextHistoryReference, 4096)
	for i := range document.HistoryRefs {
		document.HistoryRefs[i] = promptlayer.OMPContextHistoryReference{
			ID: strings.Repeat("i", 240) + fmt.Sprintf("%04d", i), SourceRef: "tool/" + strings.Repeat("s", 240),
			BodyHash: "sha256:" + strings.Repeat("d", 64), TokenEstimate: 1, Reason: "completed-superseded",
		}
	}
	require.ErrorContains(t, promptlayer.WriteOMPContextEvidenceStoreV1(t.TempDir(), document), "size limit")

	fixture := newEvidenceSecurityFixture(t)
	require.NoError(t, os.WriteFile(fixture.path, make([]byte, (1<<20)+1), 0o600))
	_, err := fixture.load(fixture.checkedAt.Add(time.Minute), fixture.expectation, fixture.policy)
	require.ErrorContains(t, err, "file size is invalid")
}

func TestOMPContextEvidenceStoreSecurity_LoadReturnsDefensiveCopies(t *testing.T) {
	fixture := newEvidenceSecurityFixture(t)
	first, err := fixture.load(fixture.checkedAt.Add(time.Minute), fixture.expectation, fixture.policy)
	require.NoError(t, err)
	artifactBefore, err := os.ReadFile(fixture.path)
	require.NoError(t, err)
	first.Promotion.Rows[0].TaskID = "mutated"
	first.HistoryRefs[0].ID = "mutated"
	second, err := fixture.load(fixture.checkedAt.Add(time.Minute), fixture.expectation, fixture.policy)
	require.NoError(t, err)
	require.NotEqual(t, "mutated", second.Promotion.Rows[0].TaskID)
	require.Equal(t, "history-1", second.HistoryRefs[0].ID)
	artifactAfter, err := os.ReadFile(fixture.path)
	require.NoError(t, err)
	require.Equal(t, artifactBefore, artifactAfter)
}

func TestOMPContextEvidenceStoreSecurity_SerializedArtifactIsBodyFreeAndPathFree(t *testing.T) {
	fixture := newEvidenceSecurityFixture(t)
	body, err := os.ReadFile(fixture.path)
	require.NoError(t, err)
	serialized := string(body)
	require.NotContains(t, serialized, fixture.root)
	for _, forbidden := range []string{`"body":`, `"content":`, `"prompt":`, "raw prompt body"} {
		require.NotContains(t, serialized, forbidden)
	}
	var document map[string]any
	require.NoError(t, json.Unmarshal(body, &document))
	require.ElementsMatch(t, []string{"schema_version", "binding", "policy", "canary_rows", "history_refs"}, mapKeys(document))
	require.ElementsMatch(t, []string{"id", "source_ref", "body_hash", "token_estimate", "reason"}, mapKeys(document["history_refs"].([]any)[0].(map[string]any)))
}

type evidenceSecurityFixture struct {
	root, path  string
	checkedAt   time.Time
	expectation promptlayer.OMPContextEvidenceExpectationV1
	policy      promptlayer.OMPContextPromotionPolicyV1
	subject     promptlayer.OMPContextPromotionSubjectV1
}

func newEvidenceSecurityFixture(t *testing.T) evidenceSecurityFixture {
	t.Helper()
	checkedAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	document := evidenceSecurityDocument(checkedAt)
	root := t.TempDir()
	require.NoError(t, promptlayer.WriteOMPContextEvidenceStoreV1(root, document))
	return evidenceSecurityFixture{
		root: root, path: promptlayer.OMPContextEvidenceStorePath(root), checkedAt: checkedAt, policy: document.Policy,
		expectation: promptlayer.OMPContextEvidenceExpectationV1{WorkspaceID: document.Binding.WorkspaceID, SpecID: document.Binding.SpecID, SnapshotHash: document.Binding.SnapshotHash, GitCommitHash: document.Binding.GitCommitHash, RuntimeVersion: document.Binding.RuntimeVersion},
		subject:     promptlayer.OMPContextPromotionSubjectV1{WorkspaceID: document.Binding.WorkspaceID, SpecID: document.Binding.SpecID, TaskID: "T5", Phase: "implementation", SessionID: "session-1", BindingHash: "sha256:" + strings.Repeat("e", 64)},
	}
}

func (f evidenceSecurityFixture) load(now time.Time, expectation promptlayer.OMPContextEvidenceExpectationV1, policy promptlayer.OMPContextPromotionPolicyV1) (promptlayer.OMPContextVerifiedEvidenceStoreV1, error) {
	return promptlayer.LoadOMPContextEvidenceForExpectationV1(f.root, expectation, f.subject, policy, now)
}

func evidenceSecurityDocument(now time.Time) promptlayer.OMPContextEvidenceStoreV1 {
	return promptlayer.OMPContextEvidenceStoreV1{
		Binding: evidenceStoreBinding(now), Policy: evidenceStorePolicy(), CanaryRows: promotionRows(),
		HistoryRefs: []promptlayer.OMPContextHistoryReference{{ID: "history-1", SourceRef: "tool/read-old", BodyHash: "sha256:" + strings.Repeat("d", 64), TokenEstimate: 1000, Reason: "completed-superseded"}},
	}
}

func mapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	return keys
}
