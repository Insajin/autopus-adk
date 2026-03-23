package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckCmd_NoFlags verifies that running `check` with no flags exits cleanly
// when there are no oversized files and the last commit (if any) is valid.
func TestCheckCmd_NoFlags(t *testing.T) {
	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--dir", t.TempDir()})

	// In a temp dir there are no .go files, so arch passes.
	// There are no commits, so lore is skipped.
	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckCmd_QuietSuppressesOutput verifies that --quiet suppresses the banner.
func TestCheckCmd_QuietSuppressesOutput(t *testing.T) {
	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--arch", "--quiet", "--dir", t.TempDir()})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(buf.String(), "Autopus") {
		t.Errorf("quiet mode should suppress the banner, got: %s", buf.String())
	}
}

// TestCheckCmd_ArchFailsOnOversizedFile verifies that a file exceeding 300 lines
// causes the arch check to return a non-zero exit.
func TestCheckCmd_ArchFailsOnOversizedFile(t *testing.T) {
	dir := t.TempDir()

	// Create a .go file with 301 lines.
	var sb strings.Builder
	sb.WriteString("package dummy\n")
	for i := 0; i < 300; i++ {
		sb.WriteString(fmt.Sprintf("// line %d\n", i))
	}
	if err := os.WriteFile(filepath.Join(dir, "big.go"), []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--arch", "--quiet", "--dir", dir})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for oversized file, got nil")
	}
}

// TestCheckCmd_ArchPassesOnSmallFile verifies that a file under 200 lines passes cleanly.
func TestCheckCmd_ArchPassesOnSmallFile(t *testing.T) {
	dir := t.TempDir()

	content := "package dummy\n// small file\n"
	if err := os.WriteFile(filepath.Join(dir, "small.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--arch", "--quiet", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCheckCmd_ArchSkipsGeneratedFiles verifies that generated files are not checked.
func TestCheckCmd_ArchSkipsGeneratedFiles(t *testing.T) {
	dir := t.TempDir()

	// Write a 301-line generated file — should be skipped.
	var sb strings.Builder
	sb.WriteString("package dummy\n")
	for i := 0; i < 300; i++ {
		sb.WriteString(fmt.Sprintf("// line %d\n", i))
	}
	for _, name := range []string{"foo_generated.go", "bar_gen.go", "baz.pb.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--arch", "--quiet", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("generated files should be skipped, got: %v", err)
	}
}

// TestCheckCmd_LoreSkipsWhenNoCommits verifies that lore check passes gracefully
// in a directory without any git history.
func TestCheckCmd_LoreSkipsWhenNoCommits(t *testing.T) {
	root := newTestRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"check", "--lore", "--dir", t.TempDir()})

	if err := root.Execute(); err != nil {
		t.Fatalf("lore check should pass when there are no commits, got: %v", err)
	}
}
