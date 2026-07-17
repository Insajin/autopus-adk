package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncVerifyPartitionsCanonicalRootPolicyAndGeneratedSurfaces(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")

	for _, rel := range []string{
		"AGENTS.md", "ARCHITECTURE.md", "CLAUDE.md", "autopus.yaml",
		"opencode.json", ".mcp.json", ".autopus/context/constraints.yaml",
		".autopus/project/workspace.md", ".autopus/specs/SPEC-META-001/spec.md",
		".autopus/specs/SPEC-META-001/plan.md",
		".autopus/learnings/pipeline.jsonl", "README.md",
	} {
		syncWrite(t, root, rel, "human managed\n")
	}
	for _, rel := range []string{
		".claude/generated.md", ".codex/generated.md", ".gemini/generated.md",
		".opencode/generated.md", ".autopus/context/signatures.md",
		".autopus/plugins/cache.json", ".autopus/brainstorms/run.md",
		".autopus/orchestra/state.json", ".autopus/runtime/state.json",
		".autopus/tool-manifest.json", ".agents/plugins/marketplace.json", "config.toml",
	} {
		syncWrite(t, root, rel, "generated\n")
	}
	syncWrite(t, root, "mystery.bin", "unknown\n")
	syncWrite(t, mod, ".codex/generated.md", "generated\n")
	syncWrite(t, mod, "pkg/owned.go", "package pkg\n")

	var out bytes.Buffer
	n, err := executeSyncVerify(&out, root, "", true)
	assert.ErrorIs(t, err, errSyncVerifyStrict)
	assert.Greater(t, n, 0)
	text := out.String()
	repos, collectErr := collectDirty(root)
	require.NoError(t, collectErr)
	classified := classifyWorkspace(repos)
	partitioned := len(classified.Blocked) + len(classified.Unclassified) + len(classified.PhaseB.Files)
	for _, group := range classified.PhaseA {
		partitioned += len(group.Files)
	}
	assert.Equal(t, len(inventoryWorkspacePaths(repos)), partitioned, "every inventory path has exactly one partition")

	for _, rel := range []string{
		"AGENTS.md", "opencode.json", ".mcp.json", ".autopus/context/constraints.yaml",
		".autopus/learnings/pipeline.jsonl", "README.md",
	} {
		assert.Contains(t, text, rel, "root keep path must be a Phase B candidate")
	}
	plan := strings.Split(text, "\nWarnings")[0]
	assert.Contains(t, plan, "git -C mod-a add -- pkg/owned.go")
	assert.NotContains(t, plan, ".codex/generated.md")
	assert.NotContains(t, plan, ".autopus/brainstorms/run.md")
	assert.NotContains(t, plan, "mystery.bin")
	assert.Contains(t, text, "blocked-path")
	assert.Contains(t, text, "unclassified-path")
}

func TestSyncVerifyIncludesTrackedButIgnoredAsBlocked(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".gitignore", ".codex/\n")
	syncWrite(t, root, ".codex/tracked.md", "baseline\n")
	syncGit(t, root, "add", ".gitignore")
	syncGit(t, root, "add", "-f", ".codex/tracked.md")
	syncGit(t, root, "commit", "-m", "tracked ignored fixture")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", true)
	assert.ErrorIs(t, err, errSyncVerifyStrict)
	assert.Contains(t, out.String(), ".codex/tracked.md")
	assert.Contains(t, out.String(), "tracked-but-ignored")
}

func TestSyncVerifyUnsafePathsNeverEnterCommands(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	unsafe := "pkg/$(touch PWNED).go"
	syncWrite(t, mod, unsafe, "package pkg\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", true)
	assert.ErrorIs(t, err, errSyncVerifyStrict)
	text := out.String()
	plan := strings.Split(text, "\nWarnings")[0]
	assert.NotContains(t, plan, unsafe)
	assert.Contains(t, text, "unsafe-plan-path")
	assert.NoFileExists(t, filepath.Join(root, "PWNED"))
}

func TestSyncVerifyRenameUsesNULTerminatedPathsAndBothSides(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, mod, "old.go", "package old\n")
	syncGit(t, mod, "add", "old.go")
	syncGit(t, mod, "commit", "-m", "old")
	require.NoError(t, os.Rename(filepath.Join(mod, "old.go"), filepath.Join(mod, "new.go")))
	syncGit(t, mod, "add", "-A")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", false)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "git -C mod-a add -- new.go")
	assert.Contains(t, out.String(), "already staged in mod-a: old.go")
}

func TestSyncVerifyUnstagedDeletionUsesUpdateAction(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, mod, "removed.go", "package removed\n")
	syncGit(t, mod, "add", "removed.go")
	syncGit(t, mod, "commit", "-m", "tracked file")
	require.NoError(t, os.Remove(filepath.Join(mod, "removed.go")))

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", false)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "git -C mod-a add -u -- removed.go")
	assert.NotContains(t, out.String(), "git -C mod-a add -- removed.go")
}

func TestSyncVerifySpecOwnsWorkspaceRelativePathsAcrossRepos(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	modA := nestedRepo(t, root, "mod-a")
	modAB := nestedRepo(t, root, "mod-ab")
	syncWrite(t, root, ".autopus/specs/SPEC-CROSS-001/spec.md", "Owns `mod-a/pkg/foo.go`.\n")
	syncWrite(t, root, ".autopus/specs/SPEC-CROSS-001/plan.md", "Implement `mod-a/pkg/foo.go`.\n")
	syncGit(t, root, "add", ".autopus/specs/SPEC-CROSS-001")
	syncGit(t, root, "commit", "-m", "cross spec")
	syncWrite(t, modA, "pkg/foo.go", "package pkg\n")
	syncWrite(t, modA, "pkg/unrelated.go", "package pkg\n")
	syncWrite(t, modAB, "pkg/foo.go", "package pkg\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "SPEC-CROSS-001", false)
	require.NoError(t, err)
	text := out.String()
	plan := strings.Split(text, "\n--spec")[0]
	assert.Contains(t, plan, "git -C mod-a add -- pkg/foo.go")
	assert.NotContains(t, plan, "pkg/unrelated.go")
	assert.NotContains(t, plan, "git -C mod-ab")
	assert.Contains(t, text, "mod-a/pkg/foo.go")
	assert.Contains(t, text, "mod-a/pkg/unrelated.go")
	assert.Contains(t, text, "mod-ab/pkg/foo.go")
}

func TestSyncVerifySpecStrictBlocksGeneratedAndUnclassifiedPaths(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".autopus/specs/SPEC-STRICT-001/spec.md", "Owns `mod-a/pkg/foo.go`.\n")
	syncWrite(t, root, ".autopus/specs/SPEC-STRICT-001/plan.md", "Implement `mod-a/pkg/foo.go`.\n")
	syncGit(t, root, "add", ".autopus/specs/SPEC-STRICT-001")
	syncGit(t, root, "commit", "-m", "strict spec")
	syncWrite(t, mod, "pkg/foo.go", "package pkg\n")
	syncWrite(t, mod, ".codex/generated.md", "generated\n")
	syncWrite(t, root, "mystery.bin", "unknown\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "SPEC-STRICT-001", true)
	assert.ErrorIs(t, err, errSyncVerifyStrict)
	text := out.String()
	plan := strings.Split(text, "\n--spec")[0]
	assert.Contains(t, plan, "git -C mod-a add -- pkg/foo.go")
	assert.NotContains(t, plan, ".codex/generated.md")
	assert.NotContains(t, plan, "mystery.bin")
	assert.Contains(t, text, "mod-a/.codex/generated.md")
	assert.Contains(t, text, "mystery.bin")
}

func TestSyncVerifySpecPathExtractionRejectsUnsafeWholeTokens(t *testing.T) {
	text := "`mod-a/pkg/good.go` `../../mod-a/pkg/traversal.go` " +
		"`/abs/mod-a/pkg/absolute.go` `..\\mod-a/pkg/backslash.go` `$HOME/mod-a/pkg/env.go`"
	assert.Equal(t, []string{"mod-a/pkg/good.go"}, extractOwnedTokens(text))

	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".autopus/specs/SPEC-TOKEN-001/spec.md", text+"\n")
	syncWrite(t, root, ".autopus/specs/SPEC-TOKEN-001/plan.md", "Implement only the declared paths.\n")
	syncGit(t, root, "add", ".autopus/specs/SPEC-TOKEN-001")
	syncGit(t, root, "commit", "-m", "token spec")
	for _, rel := range []string{"pkg/good.go", "pkg/traversal.go", "pkg/absolute.go", "pkg/backslash.go"} {
		syncWrite(t, mod, rel, "package pkg\n")
	}

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "SPEC-TOKEN-001", true)
	assert.ErrorIs(t, err, errSyncVerifyStrict)
	plan := strings.Split(out.String(), "\n--spec")[0]
	assert.Contains(t, plan, "git -C mod-a add -- pkg/good.go")
	assert.NotContains(t, plan, "pkg/traversal.go")
	assert.NotContains(t, plan, "pkg/absolute.go")
	assert.NotContains(t, plan, "pkg/backslash.go")
}

func TestSyncVerifySpecHostMustBeUniqueAndContained(t *testing.T) {
	t.Run("duplicate", func(t *testing.T) {
		root := t.TempDir()
		initSyncRepo(t, root)
		mod := nestedRepo(t, root, "mod-a")
		for _, dir := range []string{root, mod} {
			syncWrite(t, dir, ".autopus/specs/SPEC-DUP-001/spec.md", "# spec\n")
			syncWrite(t, dir, ".autopus/specs/SPEC-DUP-001/plan.md", "# plan\n")
		}
		var out bytes.Buffer
		_, err := executeSyncVerify(&out, root, "SPEC-DUP-001", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple hosts")
		assert.Empty(t, out.String())
	})

	t.Run("symlink escape", func(t *testing.T) {
		root := t.TempDir()
		initSyncRepo(t, root)
		nestedRepo(t, root, "mod-a")
		outside := t.TempDir()
		syncWrite(t, outside, "spec.md", "# outside\n")
		syncWrite(t, outside, "plan.md", "# outside\n")
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus", "specs"), 0o755))
		require.NoError(t, os.Symlink(outside, filepath.Join(root, ".autopus", "specs", "SPEC-LINK-001")))
		var out bytes.Buffer
		_, err := executeSyncVerify(&out, root, "SPEC-LINK-001", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "symlink")
		assert.NotContains(t, err.Error(), outside)
	})
}

func TestSyncVerifySpecDocumentReadErrorsFailClosed(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	nestedRepo(t, root, "mod-a")
	syncWrite(t, root, ".autopus/specs/SPEC-INCOMPLETE-001/spec.md", "# spec\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "SPEC-INCOMPLETE-001", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan.md")
	assert.NotContains(t, err.Error(), root)
}

func TestSyncVerifyPorcelainNULParserPreservesRenameAndSpecialNames(t *testing.T) {
	raw := []byte("R  new -> literal.go\x00old -> literal.go\x00?? line\nbreak.go\x00")
	files, err := parsePorcelainXY(raw)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"line\nbreak.go":    true,
		"new -> literal.go": true,
		"old -> literal.go": true,
	}, relSet(files))
	_, err = parsePorcelainXY([]byte("?? unterminated"))
	assert.Error(t, err)
}

func TestSyncVerifyGitInvocationDisablesOptionalLocksAndSanitizesErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX executable script")
	}
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "args.log")
	script := filepath.Join(bin, "git")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SYNC_GIT_LOG\"\nprintf '%s\\n' \"$SYNC_GIT_SECRET\" >&2\nexit 17\n"), 0o755))
	t.Setenv("PATH", bin)
	t.Setenv("SYNC_GIT_LOG", logPath)
	secret := filepath.Join(t.TempDir(), "TOKEN-super-secret")
	t.Setenv("SYNC_GIT_SECRET", secret)

	_, err := runSyncGit("repo with unsafe name", t.TempDir(), "status", "--porcelain=v1", "-z")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, err.Error(), "repo with unsafe name")
	assert.Contains(t, err.Error(), "<unsafe-repo>")
	args, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.True(t, strings.HasPrefix(string(args), "--no-optional-locks\nstatus\n"), string(args))
}
