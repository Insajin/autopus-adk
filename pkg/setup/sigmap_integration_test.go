//go:build integration

package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSignatureMap_CreatesFile(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	err := generateSignatureMap(projectDir, nil)
	require.NoError(t, err)

	outPath := filepath.Join(projectDir, signaturesDir, signaturesFile)
	assert.FileExists(t, outPath)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Signature Map")
}

func TestGenerateSignatureMap_DisabledByConfig(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	cfg := &config.HarnessConfig{
		Context: config.ContextConf{SignatureMap: false},
	}

	err := generateSignatureMap(projectDir, cfg)
	require.NoError(t, err)

	// File must not be created when disabled.
	outPath := filepath.Join(projectDir, signaturesDir, signaturesFile)
	assert.NoFileExists(t, outPath)
}

func TestGenerateSignatureMap_EnabledByConfig(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	cfg := &config.HarnessConfig{
		Context: config.ContextConf{SignatureMap: true},
	}

	err := generateSignatureMap(projectDir, cfg)
	require.NoError(t, err)

	outPath := filepath.Join(projectDir, signaturesDir, signaturesFile)
	assert.FileExists(t, outPath)
}

func TestUpdateSignatureMap_DetectsChange(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// First generation — no existing file, always "changed".
	updated, err := updateSignatureMap(projectDir, nil)
	require.NoError(t, err)
	assert.True(t, updated)
}

func TestUpdateSignatureMap_NoChange(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	// Generate once to create the file.
	err := generateSignatureMap(projectDir, nil)
	require.NoError(t, err)

	// Second call with identical source — should report no change.
	updated, err := updateSignatureMap(projectDir, nil)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestUpdateSignatureMap_DisabledByConfig(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	cfg := &config.HarnessConfig{
		Context: config.ContextConf{SignatureMap: false},
	}

	updated, err := updateSignatureMap(projectDir, cfg)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestGenerate_CreatesSignatureMap(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	outPath := filepath.Join(projectDir, signaturesDir, signaturesFile)
	assert.FileExists(t, outPath)
}

func TestUpdate_SignatureMapUpdatedWhenChanged(t *testing.T) {
	t.Parallel()
	projectDir := setupGoProject(t)

	_, err := Generate(projectDir, nil)
	require.NoError(t, err)

	// Add a new exported function to force sigmap change.
	writeFile(t, projectDir, "pkg/util/extra.go", "package util\n\n// NewHelper returns a helper.\nfunc NewHelper() string { return \"\" }\n")

	updated, err := Update(projectDir, "")
	require.NoError(t, err)

	found := false
	for _, name := range updated {
		if name == signaturesFile {
			found = true
			break
		}
	}
	assert.True(t, found, "signatures.md should appear in updated list")
}
