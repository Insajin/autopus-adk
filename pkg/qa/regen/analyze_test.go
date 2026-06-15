package regen

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// AC-QAMESH11-001: present surfaces are reported in the fixed order web,
// desktop, mobile, and an absent signal produces no surface.
func TestPresentSurfaces_FixedOrder(t *testing.T) {
	dir := t.TempDir()
	// Browser signal (web) + Android signal (mobile); no desktop signal.
	writeFile(t, dir, "playwright.config.ts", "export default {}")
	writeFile(t, dir, "android/app/src/main/AndroidManifest.xml", "<manifest/>")

	got := PresentSurfaces(dir)
	want := []string{SurfaceWeb, SurfaceMobile}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("surfaces = %v, want %v", got, want)
	}
}

// AnalyzeProject returns surfaces even when CLI extraction yields nothing, and
// never errors on a missing Cobra tree.
func TestAnalyzeProject_NoCLISignals(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "playwright.config.ts", "export default {}")

	analysis, err := AnalyzeProject(dir)
	if err != nil {
		t.Fatalf("AnalyzeProject: %v", err)
	}
	if !reflect.DeepEqual(analysis.Surfaces, []string{SurfaceWeb}) {
		t.Fatalf("surfaces = %v, want [web]", analysis.Surfaces)
	}
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
