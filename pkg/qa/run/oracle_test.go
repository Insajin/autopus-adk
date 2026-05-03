package run

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyOracleEvaluatesExpectedFields(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "report.txt"), []byte("ok"), 0o644))

	tests := []struct {
		name           string
		expected       map[string]any
		result         commandResult
		wantStatus     string
		wantActualPart string
		wantSummary    string
	}{
		{
			name:       "default exit zero passes",
			result:     commandResult{Status: "passed", ExitCode: 0},
			wantStatus: "passed",
		},
		{
			name:       "nonzero expected exit passes",
			expected:   map[string]any{"exit_code": 1},
			result:     commandResult{Status: "failed", ExitCode: 1},
			wantStatus: "passed",
		},
		{
			name:        "exit mismatch fails",
			expected:    map[string]any{"exit_code": 0},
			result:      commandResult{Status: "failed", ExitCode: 1},
			wantStatus:  "failed",
			wantSummary: "expected exit_code=0, got 1",
		},
		{
			name:       "stdout match passes",
			expected:   map[string]any{"stdout_contains": "needle"},
			result:     commandResult{Status: "passed", ExitCode: 0, StdoutText: "has needle"},
			wantStatus: "passed",
		},
		{
			name:           "stdout mismatch fails",
			expected:       map[string]any{"stdout_contains": "needle"},
			result:         commandResult{Status: "passed", ExitCode: 0, StdoutText: "missing"},
			wantStatus:     "failed",
			wantActualPart: "stdout_contains=false",
			wantSummary:    "stdout did not contain expected text",
		},
		{
			name:       "file exists passes",
			expected:   map[string]any{"file_exists": "report.txt"},
			result:     commandResult{Status: "passed", ExitCode: 0},
			wantStatus: "passed",
		},
		{
			name:           "file missing fails",
			expected:       map[string]any{"file_exists": "missing.txt"},
			result:         commandResult{Status: "passed", ExitCode: 0},
			wantStatus:     "failed",
			wantActualPart: "file_exists=false",
			wantSummary:    "expected file was not created",
		},
		{
			name:        "blocked result stays blocked",
			result:      commandResult{Status: "blocked", ExitCode: -1, FailureSummary: "timeout"},
			wantStatus:  "blocked",
			wantSummary: "timeout",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pack := journeyPack("go-test", "go test ./...")
			pack.Checks[0].Expected = tt.expected
			check := IndexCheck{ID: "unit"}
			result := tt.result

			applyOracle(projectDir, pack, &result, &check)

			assert.Equal(t, tt.wantStatus, result.Status)
			assert.Equal(t, tt.wantStatus, check.Status)
			if tt.wantActualPart != "" {
				assert.Contains(t, check.Actual, tt.wantActualPart)
			}
			if tt.wantSummary != "" {
				assert.Equal(t, tt.wantSummary, check.FailureSummary)
			}
		})
	}
}

func TestOracleHelperBranches(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]any{"exit_code": 0}, firstExpected(journey.Pack{}))
	assert.Equal(t, 2, expectedExitCode(map[string]any{"exit_code": int64(2)}))
	assert.Equal(t, 3, expectedExitCode(map[string]any{"exit_code": float64(3)}))
	assert.Equal(t, 4, expectedExitCode(map[string]any{"exit_code": "4"}))
	assert.Equal(t, 0, expectedExitCode(map[string]any{"exit_code": "bad"}))
	value, ok := stringExpected(map[string]any{"stdout_contains": "ok"}, "stdout_contains")
	assert.True(t, ok)
	assert.Equal(t, "ok", value)
	_, ok = stringExpected(map[string]any{"stdout_contains": ""}, "stdout_contains")
	assert.False(t, ok)
	_, ok = stringExpected(map[string]any{"stdout_contains": 1}, "stdout_contains")
	assert.False(t, ok)
	assert.Equal(t, "exit_code=0", formatExpected(map[string]any{}))

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "artifact.txt"), []byte("ok"), 0o644))
	assert.True(t, safeFileExists(dir, "artifact.txt"))
	assert.False(t, safeFileExists(dir, filepath.Join(dir, "artifact.txt")))
	assert.False(t, safeFileExists(dir, "../artifact.txt"))
}

func TestRunCommandBlocksEmptyCommandAndRecordsArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	artifactDir := filepath.Join(dir, "artifacts")

	result := runCommand(dir, journey.Pack{
		ID:      "empty",
		Adapter: journey.AdapterRef{ID: "playwright"},
		Command: journey.Command{CWD: ".", Timeout: "60s"},
	}, artifactDir)

	assert.Equal(t, "blocked", result.Status)
	assert.Equal(t, "empty command", result.FailureSummary)
	assert.FileExists(t, filepath.Join(artifactDir, "stdout.log"))
	assert.FileExists(t, filepath.Join(artifactDir, "stderr.log"))
}

func TestExitCodeFallback(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, exitCode(errors.New("not exit error")))
	assert.NotEmpty(t, finishCommandResult(commandResult{StartedAt: time.Now().UTC()}, t.TempDir(), nil, nil).StdoutPath)
}
