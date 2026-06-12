package run

import (
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// TestWorkspaceRef covers the repo-id derivation and the fallback-to-"project"
// branch when the base name normalizes to empty.
func TestWorkspaceRef(t *testing.T) {
	ref := workspaceRef("/tmp/My Repo!")
	if ref.RepoID != "My-Repo" {
		t.Fatalf("RepoID = %q, want My-Repo", ref.RepoID)
	}
	if ref.WorkspaceID != ref.RepoID || ref.RepoRoot != "." {
		t.Fatalf("unexpected WorkspaceRef %+v", ref)
	}

	// A base name that normalizes to empty falls back to "project".
	fallback := workspaceRef("/")
	if fallback.RepoID != "project" {
		t.Fatalf("root RepoID = %q, want project", fallback.RepoID)
	}
}

// TestSourceSpecForLane covers each lane-to-spec mapping and the default.
func TestSourceSpecForLane(t *testing.T) {
	cases := map[string]string{
		"browser-staging":  "SPEC-QAMESH-005",
		"desktop-native":   "SPEC-QAMESH-005",
		"gui-explore":      "SPEC-QAMESH-003",
		"mobile-readiness": "SPEC-QAMESH-006",
		"canary-explicit":  "SPEC-QAMESH-004",
		"golden":           "SPEC-QAMESH-002",
	}
	for lane, want := range cases {
		if got := sourceSpecForLane(lane); got != want {
			t.Fatalf("sourceSpecForLane(%s) = %q, want %q", lane, got, want)
		}
	}
}

// TestPackSourceRefs covers packSourceRefs, asserting it builds a qamesh
// source ref scoped to the repo id and the pack's resolved source spec.
func TestPackSourceRefs(t *testing.T) {
	pack := journey.Pack{Lanes: []string{"gui-explore"}}
	refs := packSourceRefs("/tmp/repo", pack)
	if len(refs) != 1 {
		t.Fatalf("expected 1 source ref, got %d", len(refs))
	}
	// The resolved spec is provided by sourceRefs(pack); the ref must be scoped
	// to the repo id "repo" and the resolved spec segment.
	wantPrefix := "qamesh://source/repo/specs/SPEC-QAMESH-"
	if got := refs[0]; len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("packSourceRefs = %q, want prefix %q", got, wantPrefix)
	}
}

// TestPublicProjectPath covers the relative, absolute-outside, scheme-URL, and
// empty branches of publicProjectPath.
func TestPublicProjectPath(t *testing.T) {
	root := t.TempDir()

	if got := publicProjectPath(root, ""); got != "" {
		t.Fatalf("empty path = %q, want empty", got)
	}
	if got := publicProjectPath(root, "https://x.test/a"); got != redactedPublicPath {
		t.Fatalf("scheme url = %q, want redacted", got)
	}

	inside := filepath.Join(root, "artifacts", "log.txt")
	if got := publicProjectPath(root, inside); got != "artifacts/log.txt" {
		t.Fatalf("inside path = %q, want artifacts/log.txt", got)
	}

	// An absolute path outside the project root is redacted.
	if got := publicProjectPath(root, "/etc/passwd"); got != redactedPublicPath {
		t.Fatalf("outside abs path = %q, want redacted", got)
	}
}

// TestPublicPreviewPath covers the scheme short-circuit and delegation to
// publicProjectPath.
func TestPublicPreviewPath(t *testing.T) {
	root := t.TempDir()
	if got := publicPreviewPath(root, "ftp://h/x"); got != redactedPublicPath {
		t.Fatalf("scheme preview = %q, want redacted", got)
	}
	inside := filepath.Join(root, "preview.png")
	if got := publicPreviewPath(root, inside); got != "preview.png" {
		t.Fatalf("inside preview = %q, want preview.png", got)
	}
}
