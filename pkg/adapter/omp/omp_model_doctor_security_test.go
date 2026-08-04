package omp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckOMPModelRoutingDoctor_StrictReceiptRejectsUnknownAndOversizedContent(t *testing.T) {
	t.Parallel()

	unknown := writeOMPModelDoctorFixture(t)
	path := filepath.Join(unknown.WorkspaceRoot, OMPModelReceiptRelativePath)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	payload["provider_payload"] = "must-not-be-accepted"
	data, err = json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	assert.Equal(t, "receipt_invalid", CheckOMPModelRoutingDoctor(unknown).Reason)

	trailing := writeOMPModelDoctorFixture(t)
	path = filepath.Join(trailing.WorkspaceRoot, OMPModelReceiptRelativePath)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	data = append(data, []byte("{}\n")...)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	assert.Equal(t, "receipt_invalid", CheckOMPModelRoutingDoctor(trailing).Reason)

	oversized := modelDoctorInput(t.TempDir())
	path = filepath.Join(oversized.WorkspaceRoot, OMPModelReceiptRelativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), maxOMPModelDoctorReceiptBytes+1), 0o600))
	assert.Equal(t, "receipt_invalid", CheckOMPModelRoutingDoctor(oversized).Reason)
}

func TestCheckOMPModelRoutingDoctor_RejectsSymlinkedReceiptParent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, ".autopus")))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "omp-model-resolution-v1.json"), []byte("{}"), 0o600))
	report := CheckOMPModelRoutingDoctor(modelDoctorInput(root))
	assert.Equal(t, "receipt_invalid", report.Reason)
}

func TestCheckOMPModelRoutingDoctor_RedactsUnsafeCurrentRoleFields(t *testing.T) {
	t.Parallel()

	input := writeOMPModelDoctorFixture(t)
	input.Compilation.Resolutions[0].Agent = "bad\napi_key=secret"
	report := CheckOMPModelRoutingDoctor(input)
	require.NotEmpty(t, report.Roles)
	found := false
	for _, row := range report.Roles {
		found = found || row.Agent == "redacted"
	}
	assert.True(t, found)
	encoded, err := json.Marshal(report)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "api_key")
	assert.NotContains(t, string(encoded), "secret")
}
