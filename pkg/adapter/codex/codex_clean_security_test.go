package codex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClean_RejectsManifestTraversal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("preserve"), 0o600))
	relative, err := filepath.Rel(root, victim)
	require.NoError(t, err)
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[relative] = adapter.ManifestFile{
		Checksum: adapter.Checksum("preserve"), Policy: adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err = NewWithRoot(root).Clean(context.Background())

	require.Error(t, err)
	assertFileContent(t, victim, "preserve")
}

func TestClean_RejectsManifestPathOutsideManagedNamespaces(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	victim := filepath.Join(root, "README.md")
	require.NoError(t, os.WriteFile(victim, []byte("preserve"), 0o600))
	manifest := adapter.NewManifest(adapterName)
	manifest.Files["README.md"] = adapter.ManifestFile{
		Checksum: adapter.Checksum("preserve"), Policy: adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())

	require.Error(t, err)
	assertFileContent(t, victim, "preserve")
}

func TestClean_RejectsUnknownFileInsideManagedRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(".codex", "skills", "codex-user-owned", "SKILL.md")
	absolute := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolute), 0o755))
	require.NoError(t, os.WriteFile(absolute, []byte("preserve"), 0o600))
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[path] = adapter.ManifestFile{
		Checksum: adapter.Checksum("preserve"), Policy: adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())

	require.Error(t, err)
	assertFileContent(t, absolute, "preserve")
}

func TestClean_RejectsInWorkspaceSymlinkedSpecialFiles(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, managedPath, victimBody string
	}{
		{
			name: "config", managedPath: codexConfigRelPath,
			victimBody: "approval_policy = \"on-request\"\n",
		},
		{
			name: "root document", managedPath: "AGENTS.md",
			victimBody: markerBegin + "\nmanaged\n" + markerEnd + "\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			victim := filepath.Join(root, "user-owned.txt")
			require.NoError(t, os.WriteFile(victim, []byte(test.victimBody), 0o600))
			managed := filepath.Join(root, test.managedPath)
			require.NoError(t, os.MkdirAll(filepath.Dir(managed), 0o755))
			requireSymlink(t, victim, managed)
			manifest := adapter.NewManifest(adapterName)
			manifest.Files[test.managedPath] = adapter.ManifestFile{
				Checksum: adapter.Checksum(test.victimBody), Policy: adapter.OverwriteMerge,
			}
			require.NoError(t, manifest.Save(root))

			err := NewWithRoot(root).Clean(context.Background())

			require.Error(t, err)
			assertFileContent(t, victim, test.victimBody)
		})
	}
}

func TestClean_RejectsSymlinkedManagedParent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	victim := filepath.Join(outside, "SKILL.md")
	require.NoError(t, os.WriteFile(victim, []byte("preserve"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".codex", "skills"), 0o755))
	requireSymlink(t, outside, filepath.Join(root, ".codex", "skills", "codex-auto"))
	path := filepath.Join(".codex", "skills", "codex-auto", "SKILL.md")
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[path] = adapter.ManifestFile{
		Checksum: adapter.Checksum("preserve"), Policy: adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())

	require.Error(t, err)
	assertFileContent(t, victim, "preserve")
}

func TestClean_RejectsSymlinkedPruneRootBeforeMutation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "skills"), 0o755))
	requireSymlink(t, outside, filepath.Join(root, ".codex"))
	manifest := adapter.NewManifest(adapterName)
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())

	require.Error(t, err)
	assert.DirExists(t, filepath.Join(outside, "skills"))
	assert.FileExists(t, filepath.Join(root, ".autopus", "codex-manifest.json"))
}

func TestClean_PreservesModifiedManagedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(".codex", "skills", "codex-auto", "SKILL.md")
	absolute := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolute), 0o755))
	require.NoError(t, os.WriteFile(absolute, []byte("user edit"), 0o600))
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[path] = adapter.ManifestFile{
		Checksum: adapter.Checksum("generated"), Policy: adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	require.NoError(t, NewWithRoot(root).Clean(context.Background()))

	assert.FileExists(t, absolute)
	assertFileContent(t, absolute, "user edit")
	assert.NoFileExists(t, filepath.Join(root, ".autopus", "codex-manifest.json"))
}
