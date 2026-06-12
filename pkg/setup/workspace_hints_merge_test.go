package setup

import (
	"strings"
	"testing"
)

// TestDetectGoImportStyleGrouped covers the grouped branch (blank line inside
// import block) and the ungrouped branch (no blank line).
func TestDetectGoImportStyle(t *testing.T) {
	dir := t.TempDir()

	grouped := `package main

import (
	"fmt"

	"path/filepath"
)

func main() { fmt.Println(filepath.Separator) }
`
	writeFile(t, dir, "grouped.go", grouped)
	if got := detectGoImportStyle([]string{dir + "/grouped.go"}); got != "grouped (stdlib / internal / external)" {
		t.Fatalf("grouped style = %q", got)
	}

	ungrouped := `package main

import (
	"fmt"
	"path/filepath"
)

func main() { fmt.Println(filepath.Separator) }
`
	writeFile(t, dir, "ungrouped.go", ungrouped)
	if got := detectGoImportStyle([]string{dir + "/ungrouped.go"}); got != "ungrouped" {
		t.Fatalf("ungrouped style = %q", got)
	}
}

// TestDetectGoImportStyleEdgeCases covers empty slice, unreadable file, and a
// file with no import block (should fall through to "unknown").
func TestDetectGoImportStyleEdgeCases(t *testing.T) {
	if got := detectGoImportStyle(nil); got != "unknown" {
		t.Fatalf("nil files = %q, want unknown", got)
	}
	// Non-existent file is skipped; empty slice returns "unknown".
	if got := detectGoImportStyle([]string{"/nonexistent/path.go"}); got != "unknown" {
		t.Fatalf("missing file = %q, want unknown", got)
	}
	dir := t.TempDir()
	writeFile(t, dir, "noimport.go", "package main\nfunc f() {}\n")
	if got := detectGoImportStyle([]string{dir + "/noimport.go"}); got != "unknown" {
		t.Fatalf("no import block = %q, want unknown", got)
	}
}

// TestDisplayFrameworkName covers the known display names and the default
// passthrough for an unrecognised framework.
func TestDisplayFrameworkName(t *testing.T) {
	cases := map[string]string{
		"nextjs":     "Next.js",
		"nuxtjs":     "Nuxt.js",
		"nestjs":     "NestJS",
		"fastapi":    "FastAPI",
		"react":      "React",
		"vue":        "Vue",
		"svelte":     "Svelte",
		"gin":        "Gin",
		"echo":       "Echo",
		"chi":        "Chi",
		"axum":       "Axum",
		"NEXTJS":     "Next.js",    // case-insensitive input
		"unknown-fw": "unknown-fw", // passthrough
	}
	for input, want := range cases {
		if got := displayFrameworkName(input); got != want {
			t.Fatalf("displayFrameworkName(%q) = %q, want %q", input, got, want)
		}
	}
}

// TestMergeFrameworks asserts deduplication by name and that missing entries
// are appended while existing entries are preserved.
func TestMergeFrameworks(t *testing.T) {
	dst := []Framework{{Name: "react", Version: "18"}, {Name: "vue", Version: "3"}}
	src := []Framework{{Name: "vue", Version: "3.4"}, {Name: "svelte", Version: "4"}}

	got := mergeFrameworks(dst, src)
	// "vue" must not be duplicated.
	count := 0
	for _, f := range got {
		if f.Name == "vue" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("vue appears %d times after merge, want 1", count)
	}
	// "svelte" must be appended.
	found := false
	for _, f := range got {
		if f.Name == "svelte" {
			found = true
		}
	}
	if !found {
		t.Fatal("svelte not present after merge")
	}
	if len(got) != 3 {
		t.Fatalf("merged len = %d, want 3", len(got))
	}
}

// TestBuildWorkspaceHintsNil asserts nil info returns nil hints (no panic).
func TestBuildWorkspaceHintsNil(t *testing.T) {
	if got := buildWorkspaceHints(t.TempDir(), nil); got != nil {
		t.Fatalf("nil info should return nil, got %v", got)
	}
}

// TestBuildWorkspaceHintsSingleRepo covers the no-Workspaces, no-MultiRepo
// branch yielding a single_repo hint.
func TestBuildWorkspaceHintsSingleRepo(t *testing.T) {
	dir := t.TempDir()
	hints := buildWorkspaceHints(dir, &ProjectInfo{})
	if len(hints) != 1 {
		t.Fatalf("expected 1 hint, got %d", len(hints))
	}
	if hints[0].Kind != WorkspaceHintKindSingleRepo {
		t.Fatalf("kind = %q, want single_repo", hints[0].Kind)
	}
}

// TestBuildWorkspaceHintsWorkspace covers the multi-workspace branch.
func TestBuildWorkspaceHintsWorkspace(t *testing.T) {
	dir := t.TempDir()
	info := &ProjectInfo{
		Workspaces: []Workspace{
			{Path: "pkg/a", Type: "go"},
			{Path: "pkg/b", Type: "go"},
			{Path: "pkg/c", Type: "node"},
		},
	}
	hints := buildWorkspaceHints(dir, info)
	if len(hints) != 1 || hints[0].Kind != WorkspaceHintKindWorkspace {
		t.Fatalf("kind = %q hints = %v", hints[0].Kind, hints)
	}
	if !strings.Contains(hints[0].Message, "3 modules") {
		t.Fatalf("message = %q, want '3 modules'", hints[0].Message)
	}
}

// TestBuildWorkspaceHintsMultiRepo covers the multi-repo branch, asserting the
// component count is embedded and the root repo name is used when available.
func TestBuildWorkspaceHintsMultiRepo(t *testing.T) {
	dir := t.TempDir()
	info := &ProjectInfo{
		MultiRepo: &MultiRepoInfo{
			IsMultiRepo: true,
			Components: []RepoComponent{
				{Name: "meta", Path: "."},
				{Name: "adk", Path: "adk"},
				{Name: "desktop", Path: "desktop"},
			},
		},
	}
	hints := buildWorkspaceHints(dir, info)
	if len(hints) != 1 || hints[0].Kind != WorkspaceHintKindMultiRepo {
		t.Fatalf("kind = %q", hints[0].Kind)
	}
	if !strings.Contains(hints[0].Message, "3 repos") {
		t.Fatalf("message = %q, want '3 repos'", hints[0].Message)
	}
	// Root repo name "meta" should appear in the hint.
	if hints[0].Repo != "meta" {
		t.Fatalf("repo = %q, want meta", hints[0].Repo)
	}
}
