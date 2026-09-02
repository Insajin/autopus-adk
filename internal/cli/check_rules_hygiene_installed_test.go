package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// consumerRepo builds a git repo shaped like a harness consumer: a generated
// surface and a platform manifest, and none of the ADK source directories the
// source-of-truth check looks for.
func consumerRepo(t *testing.T, generated, body string) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	target := filepath.Join(dir, filepath.FromSlash(generated))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := adapter.Manifest{
		Version:  "1.0.0",
		Platform: "codex",
		Files:    map[string]adapter.ManifestFile{generated: {Checksum: adapter.Checksum(body)}},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".autopus"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".autopus", "codex-manifest.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "add", "--", generated)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	return dir
}

// A consumer repo has no content/ or templates/ to stage, so the source-of-truth
// check can never pass there and every `auto update` result was blocked. The
// manifest claim is the locally decidable evidence that replaces it.
func TestBlockedGeneratedDrift_AllowsInstallerOutputInConsumerRepo(t *testing.T) {
	const generated = ".codex/agents/executor.toml"
	dir := consumerRepo(t, generated, "model = \"gpt-5.6-sol\"\n")

	if blocked := blockedGeneratedDrift(dir, []string{generated}); len(blocked) != 0 {
		t.Fatalf("installer output was blocked: %v", blocked)
	}
}

// The guard's purpose is unchanged: editing a generated file by hand changes the
// content without changing the recorded claim, so it still blocks.
func TestBlockedGeneratedDrift_StillBlocksHandEditedGeneratedFile(t *testing.T) {
	const generated = ".codex/agents/executor.toml"
	dir := consumerRepo(t, generated, "model = \"gpt-5.6-sol\"\n")

	target := filepath.Join(dir, filepath.FromSlash(generated))
	if err := os.WriteFile(target, []byte("model = \"hand-edited\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "--", generated)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}

	blocked := blockedGeneratedDrift(dir, []string{generated})
	if len(blocked) != 1 || blocked[0] != generated {
		t.Fatalf("hand edit was not blocked: %v", blocked)
	}
}

// Runtime state stays a hard block. A manifest that happens to claim one of
// these paths must not turn into a way to commit it.
func TestBlockedGeneratedDrift_ManifestClaimCannotClearRuntimeState(t *testing.T) {
	const generated = ".autopus/context/signatures.md"
	dir := consumerRepo(t, generated, "signature\n")

	blocked := blockedGeneratedDrift(dir, []string{generated})
	if len(blocked) != 1 || blocked[0] != generated {
		t.Fatalf("runtime state was cleared by a manifest claim: %v", blocked)
	}
}
