package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateVerifyPrimitive_ExecutesSupportedGrammar(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "artifact.txt"), []byte("state=ready\n"), 0o644))

	tests := []struct {
		name      string
		primitive string
		result    *RunnerResult
		wantPass  bool
	}{
		{
			name:      "matching exit code",
			primitive: "exit_code(0)",
			result:    &RunnerResult{ExitCode: 0, WorkDir: workDir},
			wantPass:  true,
		},
		{
			name:      "nonzero expected exit code rejects actual zero",
			primitive: "exit_code(2)",
			result:    &RunnerResult{ExitCode: 0, WorkDir: workDir},
			wantPass:  false,
		},
		{
			name:      "stdout contains JSON string",
			primitive: `stdout_contains("needle")`,
			result:    &RunnerResult{Stdout: "prefix needle suffix", WorkDir: workDir},
			wantPass:  true,
		},
		{
			name:      "stdout missing JSON string",
			primitive: `stdout_contains("missing")`,
			result:    &RunnerResult{Stdout: "needle", WorkDir: workDir},
			wantPass:  false,
		},
		{
			name:      "empty stderr",
			primitive: "stderr_empty()",
			result:    &RunnerResult{Stderr: "", WorkDir: workDir},
			wantPass:  true,
		},
		{
			name:      "nonempty stderr",
			primitive: "stderr_empty()",
			result:    &RunnerResult{Stderr: "boom", WorkDir: workDir},
			wantPass:  false,
		},
		{
			name:      "existing relative file",
			primitive: `file_exists("artifact.txt")`,
			result:    &RunnerResult{WorkDir: workDir},
			wantPass:  true,
		},
		{
			name:      "missing relative file",
			primitive: `file_exists("missing.txt")`,
			result:    &RunnerResult{WorkDir: workDir},
			wantPass:  false,
		},
		{
			name:      "relative file contains JSON string",
			primitive: `file_contains("artifact.txt", "ready")`,
			result:    &RunnerResult{WorkDir: workDir},
			wantPass:  true,
		},
		{
			name:      "relative file lacks JSON string",
			primitive: `file_contains("artifact.txt", "failed")`,
			result:    &RunnerResult{WorkDir: workDir},
			wantPass:  false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := EvaluateVerifyPrimitive(tt.primitive, tt.result)
			assert.Equal(t, tt.primitive, got.Primitive)
			assert.Equal(t, tt.wantPass, got.Pass)
			if !tt.wantPass {
				assert.NotEmpty(t, got.Message)
			}
		})
	}
}

func TestEvaluateVerifyPrimitive_UnknownOrMalformedNeverPasses(t *testing.T) {
	t.Parallel()

	tests := []string{
		"unknown_check()",
		`stdout_contains(unquoted)`,
		`file_exists("unterminated)`,
		`file_contains("only-one-argument")`,
		`file_exists("C:/Windows/system32/config")`,
		"exit_code(two)",
		"stderr_empty(extra)",
	}

	for _, primitive := range tests {
		primitive := primitive
		t.Run(primitive, func(t *testing.T) {
			t.Parallel()

			_, err := ParseVerifyPrimitive(primitive)
			require.Error(t, err)

			got := EvaluateVerifyPrimitive(primitive, &RunnerResult{ExitCode: 0, Stdout: "anything"})
			assert.False(t, got.Pass)
			assert.Equal(t, primitive, got.Primitive)
			assert.NotEmpty(t, got.Message)
		})
	}
}
