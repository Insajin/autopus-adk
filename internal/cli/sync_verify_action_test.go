package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncVerifyRenameSourceUsesIndependentStatusFlags(t *testing.T) {
	tests := []struct {
		name               string
		status             string
		oldStaged          bool
		oldUnstaged        bool
		expectedUpdated    []string
		expectedStagedOnly []string
	}{
		{name: "staged", status: "R ", oldStaged: true, expectedStagedOnly: []string{"old.go"}},
		{name: "staged destination modified", status: "RM", oldStaged: true, expectedStagedOnly: []string{"old.go"}},
		{name: "unstaged", status: " R", oldUnstaged: true, expectedUpdated: []string{"old.go"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.status + " new.go\x00old.go\x00")
			files, err := parsePorcelainXY(raw)
			require.NoError(t, err)
			require.Len(t, files, 2)

			byPath := map[string]dirtyFile{}
			for _, file := range files {
				byPath[file.Rel] = file
			}
			old := byPath["old.go"]
			assert.True(t, old.Missing)
			assert.Equal(t, tt.oldStaged, old.Staged)
			assert.Equal(t, tt.oldUnstaged, old.Unstaged)

			group := buildPhaseGroup("mod-a", []string{"new.go", "old.go"}, byPath)
			assert.Equal(t, []string{"new.go"}, group.AddFiles)
			assert.Equal(t, tt.expectedUpdated, group.UpdateFiles)
			assert.Equal(t, tt.expectedStagedOnly, group.StagedOnly)
		})
	}
}

func TestSyncVerifyStagedRenameWithModifiedDestinationDoesNotUpdateSource(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, mod, "old.go", "package old\n")
	syncGit(t, mod, "add", "old.go")
	syncGit(t, mod, "commit", "-m", "old")
	require.NoError(t, os.Rename(filepath.Join(mod, "old.go"), filepath.Join(mod, "new.go")))
	syncGit(t, mod, "add", "-A")
	syncWrite(t, mod, "new.go", "package old\n\n// modified after staging\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", false)
	require.NoError(t, err)
	plan := strings.Split(out.String(), "\nWarnings")[0]
	assert.Contains(t, plan, "git -C mod-a add -- new.go")
	assert.Contains(t, plan, "already staged in mod-a: old.go")
	assert.NotContains(t, plan, "add -u -- old.go")

	syncGit(t, mod, "add", "--", "new.go")
	assert.Contains(t, syncGitOut(t, mod, "show", ":new.go"), "modified after staging")
}

func TestSyncVerifyRecreatedStagedDeletionUsesAddAction(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, mod, "recreated.go", "package original\n")
	syncGit(t, mod, "add", "recreated.go")
	syncGit(t, mod, "commit", "-m", "original")
	syncGit(t, mod, "rm", "recreated.go")
	syncWrite(t, mod, "recreated.go", "package replacement\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", false)
	require.NoError(t, err)
	plan := strings.Split(out.String(), "\nWarnings")[0]
	assert.Contains(t, plan, "git -C mod-a add -- recreated.go")
	assert.NotContains(t, plan, "git -C mod-a add -u -- recreated.go")

	syncGit(t, mod, "add", "--", "recreated.go")
	assert.Equal(t, "package replacement\n", syncGitOut(t, mod, "show", ":recreated.go"))
}

func TestSyncVerifyRejectsPowerShellSplatPath(t *testing.T) {
	root := t.TempDir()
	initSyncRepo(t, root)
	mod := nestedRepo(t, root, "mod-a")
	syncWrite(t, mod, "@args.go", "package args\n")

	var out bytes.Buffer
	_, err := executeSyncVerify(&out, root, "", true)
	assert.ErrorIs(t, err, errSyncVerifyStrict)
	plan := strings.Split(out.String(), "\nWarnings")[0]
	assert.NotContains(t, plan, "@args.go")
	assert.Contains(t, out.String(), "unsafe-plan-path")
	assert.False(t, isSafePlanPath("@args.go"))
}
