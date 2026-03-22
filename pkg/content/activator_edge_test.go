package content

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMatch_KoreanKeyword verifies S2: Korean keyword triggers activate the correct skill.
func TestMatch_KoreanKeyword(t *testing.T) {
	t.Parallel()

	reg := &SkillRegistry{
		skills: map[string]SkillDefinition{
			"debugging": {
				Name:     "debugging",
				Category: "engineering",
				Triggers: []string{"버그", "debug", "^panic:"},
			},
		},
	}
	a := NewSkillActivator(reg, true, 10, nil)

	tests := []struct {
		name      string
		query     string
		wantMatch bool
		wantSkill string
	}{
		{"korean bug keyword", "버그 수정해줘", true, "debugging"},
		{"mixed korean english", "버그가 있어요 please debug", true, "debugging"},
		{"no match in korean", "기능 추가 요청", false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			results := a.Match(ActivationContext{UserQuery: tc.query})
			if tc.wantMatch {
				found := false
				for _, r := range results {
					if r.Skill.Name == tc.wantSkill {
						found = true
					}
				}
				if !found {
					t.Errorf("expected match for skill %q with query %q, got %v", tc.wantSkill, tc.query, results)
				}
			} else {
				if len(results) != 0 {
					t.Errorf("expected no match for query %q, got %v", tc.query, results)
				}
			}
		})
	}
}

// TestResolve_ZeroMaxActive verifies that maxActive=0 applies no cap.
func TestResolve_ZeroMaxActive(t *testing.T) {
	t.Parallel()

	reg := &SkillRegistry{
		skills: map[string]SkillDefinition{
			"a": {Name: "a", Category: "c", Triggers: []string{"foo"}},
			"b": {Name: "b", Category: "c", Triggers: []string{"foo"}},
			"c": {Name: "c", Category: "c", Triggers: []string{"foo"}},
		},
	}
	// maxActive=0 means no limit.
	a := NewSkillActivator(reg, true, 0, nil)
	ctx := ActivationContext{UserQuery: "foo"}
	results := a.Resolve(a.Match(ctx))
	if len(results) < 3 {
		t.Errorf("expected all 3 results when maxActive=0, got %d", len(results))
	}
}

// TestResolve_NoCategoryWeight verifies skills without a weight keep their original score.
func TestResolve_NoCategoryWeight(t *testing.T) {
	t.Parallel()

	reg := buildTestRegistry()
	weights := map[string]int{"engineering": 100} // only engineering gets boosted
	a := NewSkillActivator(reg, true, 10, weights)
	ctx := ActivationContext{UserQuery: "write test for handler"}
	raw := a.Match(ctx)
	resolved := a.Resolve(raw)

	for _, r := range resolved {
		if r.Skill.Category == "quality" {
			// quality has no weight — score should remain at original keyword/regex value
			if r.Score != 0.8 && r.Score != 0.9 {
				t.Errorf("expected unmodified score for quality skill, got %f", r.Score)
			}
		}
	}
}

// TestMatch_EmptyRegistry verifies Match on an empty registry returns an empty slice.
func TestMatch_EmptyRegistry(t *testing.T) {
	t.Parallel()

	reg := &SkillRegistry{skills: map[string]SkillDefinition{}}
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: "debug something"}
	results := a.Match(ctx)
	if len(results) != 0 {
		t.Errorf("expected empty results for empty registry, got %v", results)
	}
}

// TestMatch_EmptyQuery verifies Match with an empty query returns no results.
func TestMatch_EmptyQuery(t *testing.T) {
	t.Parallel()

	reg := buildTestRegistry()
	a := NewSkillActivator(reg, true, 10, nil)
	ctx := ActivationContext{UserQuery: ""}
	results := a.Match(ctx)
	if len(results) != 0 {
		t.Errorf("expected no results for empty query, got %v", results)
	}
}

// TestCollectSearchDirs_deduplication verifies that files in the same directory
// produce only one entry in the search dirs list.
func TestCollectSearchDirs_deduplication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "util.go"),
		filepath.Join(dir, "handler.go"),
	}

	dirs := collectSearchDirs(files)

	count := 0
	for _, d := range dirs {
		if d == dir {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one entry for dir %q, got %d", dir, count)
	}
}

// TestCollectSearchDirs_emptyFiles verifies that passing no files still returns cwd.
func TestCollectSearchDirs_emptyFiles(t *testing.T) {
	t.Parallel()

	dirs := collectSearchDirs(nil)
	cwd, err := os.Getwd()
	if err != nil {
		t.Skip("cannot determine cwd:", err)
	}

	found := false
	for _, d := range dirs {
		if d == cwd {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cwd %q in dirs, got %v", cwd, dirs)
	}
}

// TestDetectContext_noFiles verifies DetectContext handles an empty file list gracefully.
func TestDetectContext_noFiles(t *testing.T) {
	t.Parallel()

	ctx := DetectContext("build a feature", nil)

	if ctx.UserQuery != "build a feature" {
		t.Errorf("unexpected UserQuery: %q", ctx.UserQuery)
	}
	if len(ctx.FileExtensions) != 0 {
		t.Errorf("expected no extensions, got %v", ctx.FileExtensions)
	}
}

// TestDetectContext_multipleExtensions verifies mixed file types produce multiple extensions.
func TestDetectContext_multipleExtensions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		filepath.Join(dir, "server.go"),
		filepath.Join(dir, "frontend.js"),
		filepath.Join(dir, "styles.css"),
	}

	ctx := DetectContext("update the server", files)

	extSet := make(map[string]bool)
	for _, e := range ctx.FileExtensions {
		extSet[e] = true
	}

	if !extSet[".go"] || !extSet[".js"] || !extSet[".css"] {
		t.Errorf("expected .go, .js, .css extensions, got %v", ctx.FileExtensions)
	}
}
