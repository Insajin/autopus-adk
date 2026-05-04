package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestSpecSelfVerifyCmd_RecordsLogEntry(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "SELFVERIFY-001", "Self Verify"))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	cmd := newSpecSelfVerifyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--record", "SPEC-SELFVERIFY-001",
		"--dimension", "correctness",
		"--status", "FAIL",
		"--reason", "추상용어 미정의",
	})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "self-verify 기록 완료: SPEC-SELFVERIFY-001")

	logPath := filepath.Join(dir, ".autopus", "specs", "SPEC-SELFVERIFY-001", ".self-verify.log")
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"dimension":"correctness"`)
	assert.Contains(t, string(data), `"status":"FAIL"`)
	assert.Contains(t, string(data), `"reason":"추상용어 미정의"`)
}

func TestSpecSelfVerifyCmd_RejectsInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "SELFVERIFY-002", "Self Verify Invalid"))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	cmd := newSpecSelfVerifyCmd()
	// MAYBE is unambiguously invalid (PASS / FAIL / N/A are the only accepted
	// statuses post SPEC-SPECREV-001 follow-up).
	cmd.SetArgs([]string{
		"--record", "SPEC-SELFVERIFY-002",
		"--dimension", "correctness",
		"--status", "MAYBE",
	})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected PASS, FAIL, or N/A")

	logPath := filepath.Join(dir, ".autopus", "specs", "SPEC-SELFVERIFY-002", ".self-verify.log")
	_, statErr := os.Stat(logPath)
	assert.True(t, os.IsNotExist(statErr))
}

// TestSpecSelfVerifyCmd_AcceptsNAStatus pins the SPEC-SPECREV-001 follow-up:
// the CLI now accepts --status N/A and writes a corresponding entry.
func TestSpecSelfVerifyCmd_AcceptsNAStatus(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "SELFVERIFY-003", "Self Verify N/A"))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	cmd := newSpecSelfVerifyCmd()
	cmd.SetArgs([]string{
		"--record", "SPEC-SELFVERIFY-003",
		"--dimension", "security",
		"--status", "N/A",
		"--reason", "doc-only SPEC, no trust boundary",
	})

	require.NoError(t, cmd.Execute())

	logPath := filepath.Join(dir, ".autopus", "specs", "SPEC-SELFVERIFY-003", ".self-verify.log")
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"N/A"`)
	assert.Contains(t, string(data), `"dimension":"security"`)
}
