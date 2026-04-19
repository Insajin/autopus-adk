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
	cmd.SetArgs([]string{
		"--record", "SPEC-SELFVERIFY-002",
		"--dimension", "correctness",
		"--status", "N/A",
	})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected PASS or FAIL")

	logPath := filepath.Join(dir, ".autopus", "specs", "SPEC-SELFVERIFY-002", ".self-verify.log")
	_, statErr := os.Stat(logPath)
	assert.True(t, os.IsNotExist(statErr))
}
