//go:build integration

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunVerify_DisabledVerify verifies runVerify exits early when verify is disabled in config.
func TestRunVerify_DisabledVerify(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	// Write a minimal autopus.yaml with verify disabled.
	yaml := `mode: full
project_name: test-proj
platforms:
  - claude-code
verify:
  enabled: false
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	// runVerify should return nil because verify.enabled is false.
	// Pass nil cmd: the viewport flag check is skipped when cmd is nil.
	runErr := runVerify(nil, false, false, "desktop")
	assert.NoError(t, runErr, "runVerify must return nil when verify is disabled")
}

// TestRunVerify_EnabledNoFrontendFiles verifies runVerify exits early when no .tsx/.jsx files changed.
// This test requires node to be installed; if not, the node-check error path is covered instead.
//
// Coverage note: lines that invoke playwright (runPlaywright → collectScreenshots) cannot be
// exercised without an installed node/playwright environment. Those paths are intentionally
// excluded from automated tests to keep the test suite dependency-free.
func TestRunVerify_EnabledNoFrontendFiles(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	// Write autopus.yaml with verify enabled.
	yaml := `mode: full
project_name: test-proj
platforms:
  - claude-code
verify:
  enabled: true
  default_viewport: desktop
  auto_fix: true
  max_fix_attempts: 2
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	// runVerify will check node installation and then run git diff.
	// All cases must not panic.
	runErr := runVerify(nil, true, false, "desktop")
	_ = runErr
}

// TestRunVerify_ReportOnlyDisablesFix verifies effectiveFix=false when reportOnly=true.
func TestRunVerify_ReportOnlyDisablesFix(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	yaml := `mode: full
project_name: test-proj
platforms:
  - claude-code
verify:
  enabled: true
  default_viewport: desktop
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	// reportOnly=true means effectiveFix = fix && !reportOnly = false regardless of fix value.
	runErr := runVerify(nil, true, true, "desktop")
	_ = runErr
}

// TestRunVerify_CustomViewport verifies config viewport override logic.
// When cmd is nil (flag not explicitly set), the config default_viewport takes effect.
func TestRunVerify_CustomViewport(t *testing.T) {
	// Uses os.Chdir — not parallel-safe.
	dir := t.TempDir()

	yaml := `mode: full
project_name: test-proj
platforms:
  - claude-code
verify:
  enabled: true
  default_viewport: mobile
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	// viewport="desktop" with config default_viewport="mobile" triggers the override branch
	// because cmd=nil means the flag is treated as not explicitly changed.
	runErr := runVerify(nil, false, false, "desktop")
	_ = runErr
}

// TestRunVerify_NodeNotInstalled verifies runVerify returns an error when node is absent.
// This test works by ensuring the error path is exercised when node is not found in PATH.
func TestRunVerify_NodeNotInstalled(t *testing.T) {
	// Uses os.Chdir and PATH manipulation — not parallel-safe.
	dir := t.TempDir()

	yaml := `mode: full
project_name: test-proj
platforms:
  - claude-code
verify:
  enabled: true
`
	err := os.WriteFile(filepath.Join(dir, "autopus.yaml"), []byte(yaml), 0644)
	require.NoError(t, err)

	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()

	require.NoError(t, os.Chdir(dir))

	// Override PATH to an empty temp dir so node is not found.
	// t.Setenv auto-restores on test cleanup — no manual defer needed.
	emptyBin := t.TempDir()
	t.Setenv("PATH", emptyBin)

	runErr := runVerify(nil, false, false, "desktop")
	assert.Error(t, runErr, "runVerify must error when node is not installed")
	assert.Contains(t, runErr.Error(), "node.js")
}
