package readiness_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

// D9 regression: driving the producers with an absolute --project-dir makes them
// record absolute manifest refs. Rejecting those permanently poisons readiness
// for a project targeted the documented way, so an absolute ref that resolves
// inside the workspace root is accepted and re-published workspace-relative.
func TestProject_AcceptsAbsoluteManifestRefsInsideWorkspace(t *testing.T) {
	t.Parallel()

	root := copyFixture(t)
	absManifest := filepath.Join(root, "qa", "evidence", "manifests", "login.json")
	patchJSON(t, filepath.Join(root, "qa", "run-index.json"), func(doc map[string]any) {
		doc["manifest_paths"] = []any{absManifest}
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err != nil {
		t.Fatalf("projection blocked on the harness's own absolute path: %v", err)
	}
	refs := result.Projection.EvidenceRefs
	if len(refs) != 1 {
		t.Fatalf("evidence refs = %#v, want exactly one", refs)
	}
	if refs[0].ManifestPath != "qa/evidence/manifests/login.json" {
		t.Fatalf("manifest ref = %q, want workspace-relative", refs[0].ManifestPath)
	}
	if strings.Contains(mustJSON(t, result.Projection), root) {
		t.Fatalf("projection republished the absolute workspace path")
	}
}

// The real-world shape of D9: a project checked out under /Users/<name>/... and
// driven with an absolute --project-dir. Every producer ref then embeds a
// literal /Users/ segment and the redaction classifier rejected the whole
// document before the ref guard was even reached. Refs inside the workspace
// must project, and the published output must carry none of the local path.
func TestProject_AcceptsInWorkspaceRefsUnderAUserDirectory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "Users", "alice", "portable-shop")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	copyFixtureInto(t, root)
	patchJSON(t, filepath.Join(root, "qa", "run-index.json"), func(doc map[string]any) {
		doc["manifest_paths"] = []any{filepath.Join(root, "qa", "evidence", "manifests", "login.json")}
		doc["feedback_bundle_paths"] = []any{filepath.Join(root, "qa", "feedback", "login-codex", "bundle.json")}
	})

	result, err := readiness.Project(context.Background(), portableInput(t, root))
	if err != nil {
		t.Fatalf("projection blocked on a project living under a user directory: %v", err)
	}
	if got := result.Projection.EvidenceRefs[0].ManifestPath; got != "qa/evidence/manifests/login.json" {
		t.Fatalf("manifest ref = %q, want workspace-relative", got)
	}
	if body := mustJSON(t, result); strings.Contains(body, "/Users/alice/") {
		t.Fatalf("published output leaked the local user path: %s", body)
	}
}

// The producer and the consumer reach the same directory through different
// symlink forms: on macOS a $TMPDIR project is /var/folders/... to the shell
// that ran `auto qa release` and /private/var/folders/... to Go's os.Getwd in
// `auto qa readiness`. Resolving only the root left the two sides in opposite
// forms and every stored ref read as out-of-workspace.
func TestProject_AcceptsRefsWrittenThroughASymlinkedRoot(t *testing.T) {
	t.Parallel()

	real := filepath.Join(t.TempDir(), "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("mkdir real root: %v", err)
	}
	copyFixtureInto(t, real)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// The producer recorded refs through the symlink; the consumer is handed
	// the resolved root, exactly as os.Getwd reports it.
	patchJSON(t, filepath.Join(real, "qa", "run-index.json"), func(doc map[string]any) {
		doc["manifest_paths"] = []any{filepath.Join(link, "qa", "evidence", "manifests", "login.json")}
	})
	patchJSON(t, filepath.Join(real, "qa", "release-index.json"), func(doc map[string]any) {
		for _, row := range doc["lane_rows"].([]any) {
			entry := row.(map[string]any)
			if ref, _ := entry["run_index_path"].(string); ref != "" {
				entry["run_index_path"] = filepath.Join(link, ref)
			}
		}
	})

	result, err := readiness.Project(context.Background(), portableInput(t, real))
	if err != nil {
		t.Fatalf("projection blocked on refs written through a symlinked root: %v", err)
	}
	if got := result.Projection.EvidenceRefs[0].ManifestPath; got != "qa/evidence/manifests/login.json" {
		t.Fatalf("manifest ref = %q, want workspace-relative", got)
	}
	if body := mustJSON(t, result.Projection); strings.Contains(body, link) {
		t.Fatalf("projection republished the symlinked absolute path")
	}
}

// The normalization must not become a laundering path: an absolute ref outside
// the workspace root still fails closed, and a /Users/<name>/... leak keeps the
// class that names what it is.
func TestProject_BlocksAbsoluteManifestRefsOutsideWorkspace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ref       string
		wantClass string
	}{
		{
			name:      "local user path leak",
			ref:       "/Users/alice/private/qamesh/manifest.json",
			wantClass: "unsafe_ref:absolute_local_user_path",
		},
		{
			name:      "absolute path with no user directory",
			ref:       "/var/tmp/elsewhere/manifest.json",
			wantClass: "unsafe_ref:manifest_path_outside_workspace",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := copyFixture(t)
			patchJSON(t, filepath.Join(root, "qa", "run-index.json"), func(doc map[string]any) {
				doc["manifest_paths"] = []any{tc.ref}
			})

			result, err := readiness.Project(context.Background(), portableInput(t, root))
			if err == nil {
				t.Fatalf("projection accepted an out-of-workspace ref %q", tc.ref)
			}
			assertFailClosed(t, result, []string{tc.wantClass}, []string{tc.ref})
		})
	}
}
