package readiness_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "..", "testdata", "qa", "readiness", "non_autopus_fixture")
}

func portableInput(t *testing.T, root string) readiness.Input {
	t.Helper()
	return readiness.Input{
		WorkspaceRoot: root,
		RepoRoot:      filepath.Join(root, "repos", "portable-shop"),
		WorkspaceID:   "fixture-workspace",
		RepoID:        "portable-shop",
		RunIndexPath:  filepath.Join(root, "qa", "run-index.json"),
		ReleasePath:   filepath.Join(root, "qa", "release-index.json"),
	}
}

func assertFailClosed(t *testing.T, result readiness.Result, wantClasses, forbidden []string) {
	t.Helper()
	if result.Projection != nil || result.Rendered != nil || result.ProviderRepairPrompt != nil {
		t.Fatalf("fail-closed result produced projection/rendered/prompt output: %#v", result)
	}
	body := mustJSON(t, result.Blockers)
	for _, class := range wantClasses {
		if !strings.Contains(body, class) {
			t.Fatalf("blockers = %s, want class %q", body, class)
		}
	}
	for _, raw := range forbidden {
		if strings.Contains(body, raw) {
			t.Fatalf("blockers leaked raw value %q in %s", raw, body)
		}
	}
}

func copyFixture(t *testing.T) string {
	t.Helper()
	src := fixtureRoot(t)
	dst := t.TempDir()
	if err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, 0o644)
	}); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func patchJSON(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	var doc map[string]any
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	mutate(doc)
	next, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(next, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %#v: %v", value, err)
	}
	return string(body)
}
