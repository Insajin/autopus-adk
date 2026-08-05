package omp

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPRootedTransaction_ConfigBackupIsBoundedOwnerOnlyDelta(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	secret := []byte("providerToken: credential-must-not-be-copied\n")
	original := append(append([]byte(nil), secret...), []byte("modelRoles:\n  task: user/model\n")...)
	emitted := append(append([]byte(nil), secret...), []byte("modelRoles:\n  task: managed/model\n")...)
	require.NoError(t, workspace.atomicWrite(configFile, original, 0o640))

	journal, err := applyOMPTransactionAt(workspace, adapterName, adapter.TransactionPlan{
		Writes: []adapter.TransactionWrite{{Path: configFile, Content: emitted, Perm: 0o640}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, journal.Path)
	assert.False(t, filepath.IsAbs(journal.Path))
	assert.Equal(t, filepath.ToSlash(journal.Path), journal.Path)

	var backupPath string
	for _, entry := range journal.Entries {
		if entry.Path == configFile {
			backupPath = entry.BackupPath
		}
	}
	require.NotEmpty(t, backupPath)
	artifactData, artifactInfo, err := workspace.readOwnerOnlyFile(
		backupPath, maxOMPConfigRollbackArtifact,
	)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), artifactInfo.Mode().Perm())
	assert.LessOrEqual(t, len(artifactData), maxOMPConfigRollbackArtifact)
	assert.NotContains(t, string(artifactData), "credential-must-not-be-copied")

	artifact, err := decodeOMPConfigRollbackArtifact(artifactData)
	require.NoError(t, err)
	for _, hunk := range artifact.Hunks {
		assert.False(t, bytes.Contains(hunk.Before, []byte("credential-must-not-be-copied")))
	}
	restored, err := applyOMPConfigRollbackArtifact(artifact, emitted)
	require.NoError(t, err)
	assert.Equal(t, original, restored)

	_, journalInfo, err := workspace.readOwnerOnlyFile(journal.Path, 4<<20)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), journalInfo.Mode().Perm())
}

func TestOMPRootedTransaction_FailedConfigWriteRestoresMemoryAndDeletesDelta(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	original := []byte("providerToken: rollback-secret\nmodelRoles:\n  task: user/model\n")
	require.NoError(t, workspace.atomicWrite(configFile, original, 0o640))
	require.NoError(t, workspace.atomicWrite("blocker", []byte("not a directory\n"), 0o600))
	_, err = applyOMPTransactionAt(workspace, adapterName, adapter.TransactionPlan{Writes: []adapter.TransactionWrite{
		{Path: configFile, Content: []byte("modelRoles:\n  task: managed/model\n"), Perm: 0o640},
		{Path: "blocker/late.yml", Content: []byte("fail\n"), Perm: 0o600},
	}})
	require.Error(t, err)
	got, _, err := workspace.readFile(configFile, maxOMPConfigRollbackInputBytes)
	require.NoError(t, err)
	assert.Equal(t, original, got)
	assertNoOMPConfigRollbackArtifacts(t, root)
}

func TestOMPProjectOwnership_LegacyRawPreimageMigratesToOwnerOnlyDelta(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workspace.Close()) })

	secret := []byte("providerToken: migrate-secret\n")
	original := append(append([]byte(nil), secret...), []byte("modelRoles:\n  task: user/model\n")...)
	emitted := append(append([]byte(nil), secret...), []byte("modelRoles:\n  task: managed/model\n")...)
	ownership, _, err := newOMPModelProjectOwnership(original, false, emitted, map[string]string{})
	require.NoError(t, err)
	originLedger, err := ompModelProjectOriginLedgerBytes(ownership)
	require.NoError(t, err)
	backupPath := ".autopus/backup/legacy/.omp/config.yml"
	require.NoError(t, workspace.atomicWrite(configFile, emitted, 0o640))
	require.NoError(t, workspace.atomicWrite(backupPath, original, 0o640))
	writeOMPTransactionJournalForCoverage(t, workspace, "legacy-origin", &adapter.TransactionJournal{
		Platform: adapterName, Status: adapter.TransactionStatusCommitted,
		CreatedAt: "2026-08-05T00:00:00Z",
		Entries: []adapter.TransactionJournalEntry{
			{
				Path: OMPModelProjectOwnershipRelativePath, MissingBefore: true,
				AfterChecksum: adapter.Checksum(string(originLedger)),
			},
			{
				Path: configFile, BackupPath: backupPath, Mode: 0o640,
				AfterChecksum: adapter.Checksum(string(emitted)),
			},
		},
	})

	preimage, missing, mode, err := loadOMPModelProjectOriginalPreimageAt(workspace, ownership)
	require.NoError(t, err)
	assert.False(t, missing)
	assert.Equal(t, fs.FileMode(0o640), mode)
	assert.Equal(t, original, preimage)
	migrated, info, err := workspace.readOwnerOnlyFile(backupPath, maxOMPConfigRollbackArtifact)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), info.Mode().Perm())
	assert.NotContains(t, string(migrated), "migrate-secret")
	artifact, err := decodeOMPConfigRollbackArtifact(migrated)
	require.NoError(t, err)
	for _, hunk := range artifact.Hunks {
		assert.False(t, bytes.Contains(hunk.Before, []byte("migrate-secret")))
	}
}

func TestOMPModelIntegration_IdenticalGenerationIsByteStableAndCleansRollbackDelta(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := []byte("# user-owned credential\nproviderToken: repository-secret\n")
	configPath := filepath.Join(root, filepath.FromSlash(configFile))
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o700))
	require.NoError(t, os.WriteFile(configPath, original, 0o640))

	clock := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	a := NewWithRoot(root).
		WithModelIntegrationRunner(newModelIntegrationRunner()).
		WithModelIntegrationClock(func() time.Time { return clock })
	_, err := a.Generate(context.Background(), integrationHarnessConfig("project-managed"))
	require.NoError(t, err)
	before := snapshotTree(t, root)
	assertTreeDoesNotContainOMPSecret(t, root, []byte("repository-secret"))

	backupPath := projectConfigRollbackPath(t, root)
	require.FileExists(t, filepath.Join(root, filepath.FromSlash(backupPath)))
	clock = clock.Add(time.Hour)
	_, err = a.Generate(context.Background(), integrationHarnessConfig("project-managed"))
	require.NoError(t, err)
	assert.Equal(t, before, snapshotTree(t, root))

	require.NoError(t, a.Clean(context.Background()))
	restored, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, original, restored)
	require.NoFileExists(t, filepath.Join(root, filepath.FromSlash(backupPath)))
}

func TestOMPModelDoctorReceiptConfigSource_RequiresOwnerOnlyReceiptAndOwnership(t *testing.T) {
	t.Parallel()
	t.Run("receipt", func(t *testing.T) {
		root := t.TempDir()
		a := NewWithRoot(root).WithModelIntegrationRunner(newModelIntegrationRunner())
		_, err := a.Generate(context.Background(), integrationHarnessConfig("overlay"))
		require.NoError(t, err)
		require.NoError(t, os.Chmod(filepath.Join(root, OMPModelReceiptRelativePath), 0o644))
		_, _, reason := OMPModelDoctorReceiptConfigSource(root)
		assert.Equal(t, "receipt_invalid", reason)
	})
	t.Run("ownership", func(t *testing.T) {
		root := t.TempDir()
		a := NewWithRoot(root).WithModelIntegrationRunner(newModelIntegrationRunner())
		_, err := a.Generate(context.Background(), integrationHarnessConfig("project-managed"))
		require.NoError(t, err)
		require.NoError(t, os.Chmod(filepath.Join(root, OMPModelProjectOwnershipRelativePath), 0o644))
		_, _, reason := OMPModelDoctorReceiptConfigSource(root)
		assert.Equal(t, "receipt_invalid", reason)
	})
}

func projectConfigRollbackPath(t *testing.T, root string) string {
	t.Helper()
	workspace, err := openOMPRootedWorkspace(root)
	require.NoError(t, err)
	journals, err := loadOMPTransactionJournalsAt(workspace, adapterName)
	require.NoError(t, err)
	require.NoError(t, workspace.Close())
	for _, journal := range journals {
		for _, entry := range journal.Entries {
			if entry.Path == configFile && entry.BackupPath != "" {
				return entry.BackupPath
			}
		}
	}
	t.Fatal("project config rollback artifact not found")
	return ""
}

func assertNoOMPConfigRollbackArtifacts(t *testing.T, root string) {
	t.Helper()
	backupRoot := filepath.Join(root, ".autopus", "backup")
	err := filepath.WalkDir(backupRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Ext(path) == ".json" {
			t.Errorf("rollback artifact retained after rollback: %s", path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func assertTreeDoesNotContainOMPSecret(t *testing.T, root string, secret []byte) {
	t.Helper()
	err := filepath.WalkDir(filepath.Join(root, ".autopus"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, secret) {
			t.Errorf("credential copied into runtime artifact: %s", path)
		}
		return nil
	})
	require.NoError(t, err)
}
