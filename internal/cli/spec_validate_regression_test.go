package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecValidate_ScaffoldedSpecPasses(t *testing.T) {
	dir := t.TempDir()

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() { _ = os.Chdir(origDir) }()

	createCmd := newTestRootCmd()
	createCmd.SetArgs([]string{"spec", "new", "VALID-REG-001", "--title", "스캐폴드 검증"})
	require.NoError(t, createCmd.Execute())

	validateCmd := newTestRootCmd()
	validateCmd.SetArgs([]string{"spec", "validate", filepath.Join(dir, ".autopus", "specs", "SPEC-VALID-REG-001")})
	require.NoError(t, validateCmd.Execute())
}
