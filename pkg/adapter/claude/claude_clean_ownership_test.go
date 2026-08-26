package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestClean_RejectsManifestPathOutsideClaudeOwnership(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"README.md", ".git/hooks/post-checkout", ".claude/hooks/user-secret-scan.sh",
		".claude/skills/user-owned/SKILL.md",
	} {
		t.Run(path, func(t *testing.T) {
			root := t.TempDir()
			targetPath := filepath.Join(root, filepath.FromSlash(path))
			original := []byte("user-owned content\n")
			require.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0o755))
			require.NoError(t, os.WriteFile(targetPath, original, 0o644))

			manifest := adapter.NewManifest(adapterName)
			manifest.Files[path] = adapter.ManifestFile{
				Checksum: adapter.Checksum(string(original)),
				Policy:   adapter.OverwriteAlways,
			}
			require.NoError(t, manifest.Save(root))

			err := NewWithRoot(root).Clean(context.Background())
			require.Error(t, err)
			assert.ErrorContains(t, err, "outside Claude generated allowlist")
			assertFileContent(t, targetPath, original)
		})
	}
}

func TestClean_RejectsManifestManagedFileSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	victimPath := filepath.Join(t.TempDir(), "victim.md")
	original := []byte("external victim\n")
	require.NoError(t, os.WriteFile(victimPath, original, 0o600))

	managedPath := filepath.Join(root, ".claude", "skills", "auto", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(managedPath), 0o755))
	if err := os.Symlink(victimPath, managedPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[".claude/skills/auto/SKILL.md"] = adapter.ManifestFile{
		Checksum: adapter.Checksum(string(original)),
		Policy:   adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "crosses symlink")
	assertFileContent(t, victimPath, original)
	assert.FileExists(t, managedPath)
}

func TestClean_RejectsPermissionLedgerSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	victimPath := filepath.Join(t.TempDir(), "permission-ledger.json")
	original := []byte(`{"version":1,"allow":["UserAllow"]}` + "\n")
	require.NoError(t, os.WriteFile(victimPath, original, 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus"), 0o755))
	mustSymlink(t, victimPath, filepath.Join(root, filepath.FromSlash(claudePermissionLedgerPath)))

	manifest := adapter.NewManifest(adapterName)
	manifest.Files[claudePermissionLedgerPath] = adapter.ManifestFile{
		Checksum: adapter.Checksum(string(original)),
		Policy:   adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "crosses symlink")
	assertFileContent(t, victimPath, original)
}

func TestClean_RejectsPermissionLedgerWithoutTransactionOwnership(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	writeJSONFile(t, settingsPath, map[string]any{
		"permissions": map[string]any{"allow": []string{"UserAllow"}},
	})
	ledger := []byte(`{"version":1,"allow":["UserAllow"]}` + "\n")
	ledgerPath := filepath.Join(root, filepath.FromSlash(claudePermissionLedgerPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(ledgerPath), 0o755))
	require.NoError(t, os.WriteFile(ledgerPath, ledger, 0o600))
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[claudePermissionLedgerPath] = adapter.ManifestFile{
		Checksum: adapter.Checksum(string(ledger)),
		Policy:   adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))

	err := NewWithRoot(root).Clean(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "no committed transaction ownership")
	assert.Equal(t, []string{"UserAllow"}, permissionValues(readJSONObject(t, settingsPath), "allow"))
}

func TestClean_RejectsForgedWorldReadablePermissionLedger(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	writeJSONFile(t, settingsPath, map[string]any{
		"permissions": map[string]any{"deny": []string{"UserDeny"}},
	})
	ledger := []byte(`{"version":1,"deny":["UserDeny"]}` + "\n")
	checksum := adapter.Checksum(string(ledger))
	ledgerPath := filepath.Join(root, filepath.FromSlash(claudePermissionLedgerPath))
	require.NoError(t, os.MkdirAll(filepath.Dir(ledgerPath), 0o755))
	require.NoError(t, os.WriteFile(ledgerPath, ledger, 0o644))
	manifest := adapter.NewManifest(adapterName)
	manifest.Files[claudePermissionLedgerPath] = adapter.ManifestFile{
		Checksum: checksum,
		Policy:   adapter.OverwriteAlways,
	}
	require.NoError(t, manifest.Save(root))
	writeJSONFile(t, filepath.Join(root, ".autopus", "txns", "forged-claude", "journal.json"),
		adapter.TransactionJournal{
			ID: "forged", Platform: adapterName, Status: adapter.TransactionStatusCommitted,
			Entries: []adapter.TransactionJournalEntry{{
				Path: claudePermissionLedgerPath, Operation: "write", AfterChecksum: checksum,
			}},
		})

	err := NewWithRoot(root).Clean(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner-only regular file")
	assert.Equal(t, []string{"UserDeny"}, permissionValues(readJSONObject(t, settingsPath), "deny"))
}

func TestClean_RejectsManifestAndSharedPathSymlinks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(*testing.T, string) (string, []byte)
	}{
		{name: "manifest final", setup: setupClaudeManifestFinalSymlink},
		{name: "manifest intermediate", setup: setupClaudeManifestIntermediateSymlink},
		{name: "settings final", setup: setupClaudeSettingsFinalSymlink},
		{name: "settings intermediate", setup: setupClaudeSettingsIntermediateSymlink},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			victimPath, original := test.setup(t, root)

			err := NewWithRoot(root).Clean(context.Background())
			require.Error(t, err)
			assert.ErrorContains(t, err, "crosses symlink")
			assertFileContent(t, victimPath, original)
		})
	}
}

func TestClean_RejectsSymlinkedStatusLinePreimageWithoutCopyingVictim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	writeJSONFile(t, settingsPath, map[string]any{
		"statusLine": map[string]any{
			"type": "command", "command": autopusClaudeCombinedStatusLineCommand,
		},
	})
	before, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	victimPath := filepath.Join(t.TempDir(), "credential.txt")
	secret := []byte("external-secret\n")
	require.NoError(t, os.WriteFile(victimPath, secret, 0o600))
	mustSymlink(t, victimPath, filepath.Join(root, ".claude", "statusline-user-command.txt"))

	err = NewWithRoot(root).Clean(context.Background())
	require.Error(t, err)
	assert.ErrorContains(t, err, "crosses symlink")
	assertFileContent(t, victimPath, secret)
	assertFileContent(t, settingsPath, before)
}

func TestClean_PermissionLedgerPreservesUserDuplicatesAndRemovesActualAdds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	writeJSONFile(t, settingsPath, map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Bash(git *)", "WebSearch", "UserAllow"},
			"deny":  []string{"Bash(rm -rf:*)", "UserDeny"},
		},
	})
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/ledger\n"), 0o644))

	cfg := config.DefaultFullConfig("permission-ledger")
	cfg.Hooks.Permissions.ExtraAllow = []string{"Bash(git *)", "WebSearch", "Bash(custom:*)"}
	cfg.Hooks.Permissions.ExtraDeny = []string{"Bash(rm -rf:*)", "Bash(chmod:*)"}
	a := NewWithRoot(root)
	_, err := a.Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(root, filepath.FromSlash(claudePermissionLedgerPath)))
	ledgerInfo, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(claudePermissionLedgerPath)))
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o600), ledgerInfo.Mode().Perm())
	generated := readJSONObject(t, settingsPath)
	assert.Contains(t, permissionValues(generated, "allow"), "Bash(go build:*)")
	assert.Contains(t, permissionValues(generated, "allow"), "Bash(custom:*)")
	assert.Contains(t, permissionValues(generated, "deny"), "Bash(chmod:*)")

	require.NoError(t, os.Remove(filepath.Join(root, "go.mod")))
	require.NoError(t, a.Clean(context.Background()))

	cleaned := readJSONObject(t, settingsPath)
	assert.Equal(t, []string{"Bash(git *)", "WebSearch", "UserAllow"}, permissionValues(cleaned, "allow"))
	assert.Equal(t, []string{"Bash(rm -rf:*)", "UserDeny"}, permissionValues(cleaned, "deny"))
	assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(claudePermissionLedgerPath)))
}

func setupClaudeManifestFinalSymlink(t *testing.T, root string) (string, []byte) {
	t.Helper()
	original := []byte("{}\n")
	victimPath := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(victimPath, original, 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".autopus"), 0o755))
	mustSymlink(t, victimPath, filepath.Join(root, ".autopus", adapterName+"-manifest.json"))
	return victimPath, original
}

func setupClaudeManifestIntermediateSymlink(t *testing.T, root string) (string, []byte) {
	t.Helper()
	outside := t.TempDir()
	original := []byte("{}\n")
	victimPath := filepath.Join(outside, adapterName+"-manifest.json")
	require.NoError(t, os.WriteFile(victimPath, original, 0o600))
	mustSymlink(t, outside, filepath.Join(root, ".autopus"))
	return victimPath, original
}

func setupClaudeSettingsFinalSymlink(t *testing.T, root string) (string, []byte) {
	t.Helper()
	original := []byte("{\"permissions\":{\"allow\":[\"WebSearch\"]}}\n")
	victimPath := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(victimPath, original, 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".claude"), 0o755))
	mustSymlink(t, victimPath, filepath.Join(root, ".claude", "settings.json"))
	return victimPath, original
}

func setupClaudeSettingsIntermediateSymlink(t *testing.T, root string) (string, []byte) {
	t.Helper()
	outside := t.TempDir()
	original := []byte("{\"permissions\":{\"allow\":[\"WebSearch\"]}}\n")
	victimPath := filepath.Join(outside, "settings.json")
	require.NoError(t, os.WriteFile(victimPath, original, 0o600))
	mustSymlink(t, outside, filepath.Join(root, ".claude"))
	return victimPath, original
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func permissionValues(settings map[string]any, key string) []string {
	permissions, _ := settings["permissions"].(map[string]any)
	values, _ := permissions[key].([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
