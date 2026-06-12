package content

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkillFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLoadSkillCatalog_FromDisk(t *testing.T) {
	dir := t.TempDir()
	writeSkillFile(t, dir, "planning.md", `---
name: planning
description: Planning skill
visibility: shared
platforms:
  - codex
---
# Planning
`)
	writeSkillFile(t, dir, "metrics.md", `---
name: metrics
description: Metrics skill
visibility: shared
platforms:
  - opencode
---
# Metrics
`)
	// Non-markdown and directory entries must be ignored.
	writeSkillFile(t, dir, "ignore.txt", "not a skill")
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	catalog, err := LoadSkillCatalog(dir)
	if err != nil {
		t.Fatalf("LoadSkillCatalog: %v", err)
	}

	// List must return both skills sorted by name.
	list := catalog.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}
	if list[0].Name != "metrics" || list[1].Name != "planning" {
		t.Errorf("List order = [%s,%s], want [metrics,planning]", list[0].Name, list[1].Name)
	}

	// Get returns the parsed skill with its description.
	got, ok := catalog.Get("planning")
	if !ok {
		t.Fatal("Get(planning) not found")
	}
	if got.Description != "Planning skill" {
		t.Errorf("planning description = %q", got.Description)
	}
}

func TestLoadSkillCatalog_MissingDir(t *testing.T) {
	_, err := LoadSkillCatalog(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestSkillCatalog_NilReceivers(t *testing.T) {
	var c *SkillCatalog
	if got := c.List(); got != nil {
		t.Errorf("nil List = %v, want nil", got)
	}
	if _, ok := c.Get("x"); ok {
		t.Error("nil Get must return false")
	}
}
