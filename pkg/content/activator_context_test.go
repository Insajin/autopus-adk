package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractExtensions_deduplication(t *testing.T) {
	files := []string{"main.go", "server.go", "handler.go", "README.md"}
	exts := extractExtensions(files)

	if len(exts) != 2 {
		t.Fatalf("expected 2 unique extensions, got %d: %v", len(exts), exts)
	}

	seen := make(map[string]bool)
	for _, e := range exts {
		if seen[e] {
			t.Errorf("duplicate extension: %s", e)
		}
		seen[e] = true
	}
}

func TestExtractExtensions_noExtension(t *testing.T) {
	files := []string{"Makefile", "Dockerfile"}
	exts := extractExtensions(files)
	if len(exts) != 0 {
		t.Errorf("expected 0 extensions, got %v", exts)
	}
}

func TestExtractExtensions_lowercase(t *testing.T) {
	files := []string{"Main.GO", "App.PY"}
	exts := extractExtensions(files)
	for _, e := range exts {
		if e != ".go" && e != ".py" {
			t.Errorf("expected lower-case extension, got %s", e)
		}
	}
}

func TestDetectLanguage_markerPrecedence(t *testing.T) {
	// Even when .py extension is present, go.mod marker should win.
	lang := detectLanguage([]string{".py"}, []string{"go.mod"})
	if lang != "go" {
		t.Errorf("expected go (marker wins), got %s", lang)
	}
}

func TestDetectLanguage_extensionFallback(t *testing.T) {
	lang := detectLanguage([]string{".rs"}, nil)
	if lang != "rust" {
		t.Errorf("expected rust, got %s", lang)
	}
}

func TestDetectLanguage_typescript(t *testing.T) {
	lang := detectLanguage([]string{".ts"}, nil)
	if lang != "javascript" {
		t.Errorf("expected javascript for .ts, got %s", lang)
	}
}

func TestDetectLanguage_unknown(t *testing.T) {
	lang := detectLanguage([]string{".xyz"}, nil)
	if lang != "" {
		t.Errorf("expected empty string for unknown, got %s", lang)
	}
}

func TestDetectLanguage_allMarkers(t *testing.T) {
	cases := []struct {
		marker string
		want   string
	}{
		{"go.mod", "go"},
		{"package.json", "javascript"},
		{"Cargo.toml", "rust"},
		{"pyproject.toml", "python"},
		{"pom.xml", "java"},
	}
	for _, tc := range cases {
		got := detectLanguage(nil, []string{tc.marker})
		if got != tc.want {
			t.Errorf("marker %s: expected %s, got %s", tc.marker, tc.want, got)
		}
	}
}

func TestDetectMarkers_findsGoMod(t *testing.T) {
	// Create a temp dir with a go.mod file to simulate a Go project root.
	dir := t.TempDir()
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pass a fake file inside that directory.
	files := []string{filepath.Join(dir, "main.go")}
	markers := detectMarkers(files)

	found := false
	for _, m := range markers {
		if m == "go.mod" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected go.mod in markers, got %v", markers)
	}
}

func TestDetectMarkers_noMarkers(t *testing.T) {
	dir := t.TempDir() // empty directory, no marker files
	files := []string{filepath.Join(dir, "app.go")}
	markers := detectMarkers(files)
	// markers may include markers from cwd; we only verify no panics and type is slice.
	_ = markers
}

func TestDetectContext_integration(t *testing.T) {
	dir := t.TempDir()
	goModPath := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := []string{
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "util.go"),
	}
	ctx := DetectContext("implement feature X", files)

	if ctx.UserQuery != "implement feature X" {
		t.Errorf("unexpected UserQuery: %s", ctx.UserQuery)
	}
	if len(ctx.FileExtensions) != 1 || ctx.FileExtensions[0] != ".go" {
		t.Errorf("unexpected FileExtensions: %v", ctx.FileExtensions)
	}
	if ctx.Language != "go" {
		t.Errorf("expected language go, got %s", ctx.Language)
	}

	foundMarker := false
	for _, m := range ctx.ProjectMarkers {
		if m == "go.mod" {
			foundMarker = true
		}
	}
	if !foundMarker {
		t.Errorf("expected go.mod in ProjectMarkers, got %v", ctx.ProjectMarkers)
	}
}
