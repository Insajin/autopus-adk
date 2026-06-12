package setup

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScanRepoComponentGoModule covers ScanRepoComponent across a Go module
// repository with a git remote, asserting derived fields are populated.
func TestScanRepoComponentGoModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/adk\n\ngo 1.22\n")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	writeFile(t, dir, ".git/config", "[remote \"origin\"]\n\turl = git@github.com:example/adk.git\n")

	comp, err := ScanRepoComponent(dir)
	if err != nil {
		t.Fatalf("ScanRepoComponent returned error: %v", err)
	}
	if comp.ModulePath != "github.com/example/adk" {
		t.Fatalf("ModulePath = %q, want github.com/example/adk", comp.ModulePath)
	}
	if comp.RemoteURL != "git@github.com:example/adk.git" {
		t.Fatalf("RemoteURL = %q", comp.RemoteURL)
	}
	if comp.PrimaryLanguage != "Go" {
		t.Fatalf("PrimaryLanguage = %q, want Go", comp.PrimaryLanguage)
	}
	// Name "adk" routes inferRepoRole to the CLI/harness branch.
	if comp.Role != "CLI and harness source" {
		t.Fatalf("Role = %q, want CLI and harness source", comp.Role)
	}
	// Path is the base name when no root is supplied.
	if comp.Path != filepath.Base(comp.AbsPath) {
		t.Fatalf("Path = %q, want %q", comp.Path, filepath.Base(comp.AbsPath))
	}
}

// TestScanRepoComponentNoRemote asserts an empty remote when no git config
// exists and that a non-language repo still resolves a role.
func TestScanRepoComponentNoRemote(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# desktop app\n")

	comp, err := ScanRepoComponent(dir)
	if err != nil {
		t.Fatalf("ScanRepoComponent returned error: %v", err)
	}
	if comp.RemoteURL != "" {
		t.Fatalf("RemoteURL = %q, want empty", comp.RemoteURL)
	}
	if comp.ModulePath != "" {
		t.Fatalf("ModulePath = %q, want empty", comp.ModulePath)
	}
}

// TestReadGitRemoteGitdirFile covers the gitdir-file branch of readGitRemote,
// where .git is a file pointing to the real git directory (worktree layout).
func TestReadGitRemoteGitdirFile(t *testing.T) {
	dir := t.TempDir()
	realGit := filepath.Join(dir, "realgit")
	writeFile(t, realGit, "config", "[remote \"origin\"]\n\turl = https://example.com/r.git\n")
	// .git is a file referencing the real git dir via absolute path.
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+realGit+"\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	got := readGitRemote(dir)
	if got != "https://example.com/r.git" {
		t.Fatalf("readGitRemote = %q, want https://example.com/r.git", got)
	}
}

// TestReadGitRemoteMissing asserts an empty string when no .git exists.
func TestReadGitRemoteMissing(t *testing.T) {
	if got := readGitRemote(t.TempDir()); got != "" {
		t.Fatalf("readGitRemote on bare dir = %q, want empty", got)
	}
}

// TestInferRepoRole exercises the role classification branches.
func TestInferRepoRole(t *testing.T) {
	cases := []struct {
		name string
		comp RepoComponent
		want string
	}{
		{"meta", RepoComponent{Path: "."}, "meta workspace"},
		{"desktop", RepoComponent{Name: "autopus-desktop", Path: "desktop"}, "desktop shell"},
		{"protocol", RepoComponent{Name: "agent-protocol"}, "shared protocol"},
		{"docs", RepoComponent{Name: "docs-site"}, "documentation"},
		{"frontend", RepoComponent{Name: "web-frontend"}, "web application"},
		{"backend", RepoComponent{Name: "api-server"}, "backend service"},
		{"tap", RepoComponent{Name: "homebrew-tap"}, "distribution"},
		{"lang", RepoComponent{Name: "lib", PrimaryLanguage: "Rust"}, "Rust repository"},
		{"fallback", RepoComponent{Name: "misc", PrimaryLanguage: "Unknown"}, "repository"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := inferRepoRole(c.comp); got != c.want {
				t.Fatalf("inferRepoRole(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}
