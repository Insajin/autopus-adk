package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLSPAdapter creates a lspClientAdapter with a nil client for unit testing.
// The adapter methods return stub errors without calling the underlying LSP client.
func newTestLSPAdapter() *lspClientAdapter {
	return &lspClientAdapter{client: nil}
}

// TestLSPClientAdapter_Diagnostics verifies that Diagnostics returns a stub error.
func TestLSPClientAdapter_Diagnostics(t *testing.T) {
	t.Parallel()

	a := newTestLSPAdapter()
	result, err := a.Diagnostics("/some/path.go")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "/some/path.go"),
		"error must mention the path argument")
}

// TestLSPClientAdapter_References verifies that References returns a stub error.
func TestLSPClientAdapter_References(t *testing.T) {
	t.Parallel()

	a := newTestLSPAdapter()
	result, err := a.References("MySymbol")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MySymbol")
}

// TestLSPClientAdapter_Rename verifies that Rename returns a stub error.
func TestLSPClientAdapter_Rename(t *testing.T) {
	t.Parallel()

	a := newTestLSPAdapter()
	err := a.Rename("OldName", "NewName")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OldName")
	assert.Contains(t, err.Error(), "NewName")
}

// TestLSPClientAdapter_Symbols verifies that Symbols returns a stub error.
func TestLSPClientAdapter_Symbols(t *testing.T) {
	t.Parallel()

	a := newTestLSPAdapter()
	result, err := a.Symbols("/src/main.go")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "/src/main.go")
}

// TestLSPClientAdapter_Definition verifies that Definition returns a stub error.
func TestLSPClientAdapter_Definition(t *testing.T) {
	t.Parallel()

	a := newTestLSPAdapter()
	result, err := a.Definition("SomeFunc")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SomeFunc")
}

// TestNewLSPSubcmds_ArgValidation verifies arg count validation for each LSP subcommand.
func TestNewLSPSubcmds_ArgValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmdFn    func() interface{ Args(cmd interface{}, args []string) error }
		goodArgs []string
		badArgs  []string
	}{}

	// Validate each command's Args constraint separately.
	t.Run("diagnostics requires 1 arg", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPDiagnosticsCmd()
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"file.go"}))
	})
	t.Run("refs requires 1 arg", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPRefsCmd()
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"Symbol"}))
	})
	t.Run("rename requires 2 args", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPRenameCmd()
		assert.Error(t, cmd.Args(cmd, []string{"only-one"}))
		assert.NoError(t, cmd.Args(cmd, []string{"Old", "New"}))
	})
	t.Run("symbols requires 1 arg", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPSymbolsCmd()
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"file.go"}))
	})
	t.Run("definition requires 1 arg", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPDefinitionCmd()
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"Symbol"}))
	})

	_ = tests
}

// TestLSPCmds_FailWhenNoServer verifies that each LSP command returns error
// when createLSPClient fails (no LSP server available in test environment).
func TestLSPCmds_FailWhenNoServer(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	t.Run("diagnostics fails without server", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPDiagnosticsCmd()
		err := cmd.RunE(cmd, []string{dir + "/test.go"})
		assert.Error(t, err, "should fail without LSP server")
	})

	t.Run("refs fails without server", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPRefsCmd()
		err := cmd.RunE(cmd, []string{"SomeSymbol"})
		assert.Error(t, err, "should fail without LSP server")
	})

	t.Run("rename fails without server", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPRenameCmd()
		err := cmd.RunE(cmd, []string{"Old", "New"})
		assert.Error(t, err, "should fail without LSP server")
	})

	t.Run("symbols fails without server", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPSymbolsCmd()
		err := cmd.RunE(cmd, []string{dir + "/file.go"})
		assert.Error(t, err, "should fail without LSP server")
	})

	t.Run("definition fails without server", func(t *testing.T) {
		t.Parallel()
		cmd := newLSPDefinitionCmd()
		err := cmd.RunE(cmd, []string{"SomeFunc"})
		assert.Error(t, err, "should fail without LSP server")
	})
}
