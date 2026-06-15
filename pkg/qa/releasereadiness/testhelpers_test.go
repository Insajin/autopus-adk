package releasereadiness

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
	"gopkg.in/yaml.v3"
)

// writeSignal writes a surface signal file relative to root, creating parents.
func writeSignal(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// webSignals plants a present web surface (playwright config).
func webSignals(t *testing.T, root string) {
	t.Helper()
	writeSignal(t, root, "playwright.config.ts", "export default {};\n")
}

// desktopSignals plants a present desktop surface (Tauri config).
func desktopSignals(t *testing.T, root string) {
	t.Helper()
	writeSignal(t, root, "src-tauri/Cargo.toml", "[package]\nname = \"app\"\n")
}

// mobileSignals plants a present mobile surface (Android build.gradle).
func mobileSignals(t *testing.T, root string) {
	t.Helper()
	writeSignal(t, root, "android/app/build.gradle", "apply plugin: 'com.android.application'\n")
}

// writePack persists a journey pack YAML under the project journeys dir.
func writePack(t *testing.T, root string, pack journey.Pack) {
	t.Helper()
	dir := filepath.Join(root, ".autopus", "qa", "journeys")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir journeys: %v", err)
	}
	body, err := yaml.Marshal(pack)
	if err != nil {
		t.Fatalf("marshal pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, pack.ID+".yaml"), body, 0o644); err != nil {
		t.Fatalf("write pack: %v", err)
	}
}

// customPack builds a custom-command pack with a deterministic argv on a lane.
// custom-command validates arbitrary argv and declares no required binaries, so
// it runs hermetically through qarun.Execute with exit-derived status.
func customPack(id, surface, lane string, argv []string) journey.Pack {
	return journey.Pack{
		ID:      id,
		Title:   id,
		Surface: surface,
		Lanes:   []string{lane},
		Adapter: journey.AdapterRef{ID: "custom-command"},
		Command: journey.Command{Argv: argv, CWD: ".", Timeout: "60s"},
		Checks: []journey.Check{{
			ID:       id + "-check",
			Type:     "deterministic",
			Expected: map[string]any{"exit_code": 0},
		}},
		SourceRefs: journey.SourceRefs{
			SourceSpec:     "SPEC-QAMESH-011",
			AcceptanceRefs: []string{"AC-QAMESH11-009"},
			OwnedPaths:     []string{"."},
		},
	}
}

// fakeRun returns a runFunc that yields a fixed status, for dispatch-layer
// tests that exercise mapping without spawning a process.
func fakeRun(status string) runFunc {
	return func(qarun.Options) (qarun.Result, error) {
		return qarun.Result{Status: status, AdapterResults: []qarun.AdapterResult{}}, nil
	}
}

// realRun is the production seam: dispatch through qarun.Execute.
func realRun(o qarun.Options) (qarun.Result, error) {
	return qarun.Execute(o)
}

// withPATH sets PATH to only the given dirs for the duration of the test, so
// exec.LookPath resolves deterministically (e.g. maestro absent).
func withPATH(t *testing.T, dirs ...string) {
	t.Helper()
	prev := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", prev) })
	joined := ""
	for i, d := range dirs {
		if i > 0 {
			joined += string(os.PathListSeparator)
		}
		joined += d
	}
	if err := os.Setenv("PATH", joined); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
}

// fakeBin writes an executable shell stub returning the given exit code, used to
// satisfy a tool probe hermetically.
func fakeBin(t *testing.T, dir, name string, exitCode int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir bindir: %v", err)
	}
	script := "#!/bin/sh\nexit " + strconv.Itoa(exitCode) + "\n"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}
}
