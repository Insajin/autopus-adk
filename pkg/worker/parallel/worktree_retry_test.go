package parallel

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorktreeManager_CreateRetriesSharedLockThenSucceeds(t *testing.T) {
	t.Parallel()

	tmpDir := realTempDir(t)
	initGitRepo(t, tmpDir)

	m := NewWorktreeManager(tmpDir)
	base := time.Millisecond
	var delays []time.Duration
	attempts := 0
	m.runCommand = func(dir, name string, args ...string) (worktreeCommandResult, error) {
		attempts++
		if attempts == 1 {
			return worktreeCommandResult{stderr: []byte("fatal: Unable to create '.git/refs/heads/x.lock': File exists")}, errors.New("exit status 128")
		}
		return runGitCommand(dir, name, args...)
	}
	m.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}
	m.lockRetryBase = base

	wtPath, err := m.Create("lock-success")

	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, []time.Duration{base}, delays)
	require.NoError(t, m.Remove(wtPath, false))
}

func TestWorktreeManager_CreateExhaustsSharedLockRetries(t *testing.T) {
	t.Parallel()

	m := NewWorktreeManager(t.TempDir())
	base := time.Millisecond
	var delays []time.Duration
	attempts := 0
	m.runCommand = func(string, string, ...string) (worktreeCommandResult, error) {
		attempts++
		return worktreeCommandResult{stderr: []byte("fatal: Unable to create '.git/refs/heads/x.lock': File exists")}, errors.New("exit status 128")
	}
	m.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}
	m.lockRetryBase = base

	_, err := m.Create("persistent-lock")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "refs/heads/x.lock")
	assert.Equal(t, 4, attempts)
	assert.Equal(t, []time.Duration{base, base * 2, base * 4}, delays)
}

func TestWorktreeManager_CreateDoesNotRetryNonLockError(t *testing.T) {
	t.Parallel()

	m := NewWorktreeManager(t.TempDir())
	baseErr := errors.New("exit status 128")
	attempts := 0
	var delays []time.Duration
	m.runCommand = func(string, string, ...string) (worktreeCommandResult, error) {
		attempts++
		return worktreeCommandResult{
			stdout: []byte("stdout mentions worker-index.lock file exists"),
			stderr: []byte("fatal: invalid reference: badref"),
		}, baseErr
	}
	m.sleep = func(delay time.Duration) {
		delays = append(delays, delay)
	}
	m.lockRetryBase = time.Millisecond

	_, err := m.Create("index.lock")

	require.Error(t, err)
	assert.ErrorIs(t, err, baseErr)
	assert.Contains(t, err.Error(), "invalid reference")
	assert.Equal(t, 1, attempts)
	assert.Empty(t, delays)
}

func TestWorktreeManager_CreateRejectsUnsafeTaskID(t *testing.T) {
	t.Parallel()

	m := NewWorktreeManager(t.TempDir())
	attempts := 0
	m.runCommand = func(string, string, ...string) (worktreeCommandResult, error) {
		attempts++
		return worktreeCommandResult{}, nil
	}

	_, err := m.Create("index.lock file exists")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "task ID")
	assert.Equal(t, 0, attempts)
}

func TestWorktreeManager_RetryInjectionDefaults(t *testing.T) {
	t.Parallel()

	var m WorktreeManager

	assert.NotNil(t, m.commandRunner())
	assert.NotNil(t, m.retrySleep())
	assert.Equal(t, lockRetryBase, m.retryBase())
	assert.False(t, isLockError(nil))
	assert.False(t, isLockError(errors.New("worktree create index.lock: fatal: invalid reference: exit status 128")))
	assert.False(t, isLockError(&worktreeCreateError{
		taskID: "index.lock",
		stderr: "fatal: invalid reference: badref",
		err:    errors.New("exit status 128"),
	}))
}

func TestWorktreeRetry_SourceCount_DocumentsSingleImplementation(t *testing.T) {
	t.Parallel()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	counts, err := countWorktreeRetryImplementations(moduleRoot)
	require.NoError(t, err)

	assert.Equal(t, 1, counts.retry)
	assert.Equal(t, 1, counts.classifier)
}

type worktreeRetryCounts struct {
	retry      int
	classifier int
}

func countWorktreeRetryImplementations(moduleRoot string) (worktreeRetryCounts, error) {
	var counts worktreeRetryCounts
	retryPattern := "func " + "retryOnLock("
	classifierPattern := "func " + "isLockError("

	err := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		counts.retry += strings.Count(string(content), retryPattern)
		counts.classifier += strings.Count(string(content), classifierPattern)
		return nil
	})
	return counts, err
}
