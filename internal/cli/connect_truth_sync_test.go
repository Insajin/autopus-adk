package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectCmd_RegistersStatusSubcommand(t *testing.T) {
	t.Parallel()

	cmd := newConnectCmd()
	assertConnectStateMachineCopy(t, cmd.Long)

	assert.Contains(t, cmd.Long, "`autopus-desktop-runtime connect`")
	assert.Contains(t, cmd.Long, "`auto connect` is a retained compatibility shim")
	assert.Contains(t, cmd.Long, "delegates to the desktop-owned runtime helper.")
	assert.NotContains(t, cmd.Long, "when available")

	names := make([]string, 0, len(cmd.Commands()))
	for _, subcmd := range cmd.Commands() {
		names = append(names, subcmd.Name())
	}
	assert.Contains(t, names, "status")
}

func TestConnectStatusCmd_NotConfiguredOutput(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv(runtimeHelperOverrideEnv, writeRuntimeHelperScript(t, "cat <<'EOF'\n"+
		"Ready: false\n"+
		"Configured: false\n"+
		"Next: Use the desktop app Connect action or run `autopus-desktop-runtime connect` to authenticate with the server and select a workspace.\n"+
		"EOF"))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"connect", "status"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Ready: false")
	assert.Contains(t, out.String(), "`autopus-desktop-runtime connect`")
}

func TestConnectDocsStayInSync(t *testing.T) {
	t.Parallel()

	assertConnectStateMachineCopy(t, newConnectCmd().Long)
}

func assertConnectStateMachineCopy(t *testing.T, text string) {
	t.Helper()

	assert.Contains(t, text, "server auth → workspace → OpenAI OAuth")
	assert.Contains(t, text, "autopus-desktop-runtime connect")
	assert.NotContains(t, text, "detect → configure → verify")
	assert.NotContains(t, text, "감지 → 설정 → 검증")
}
