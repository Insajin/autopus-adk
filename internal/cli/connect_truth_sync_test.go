package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectCmd_RegistersStatusSubcommand(t *testing.T) {
	t.Parallel()

	cmd := newConnectCmd()
	assertConnectStateMachineCopy(t, cmd.Long)

	assert.Contains(t, cmd.Long, "auto connect status")

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
		"Next: Run `auto connect` to authenticate with the server and select a workspace.\n"+
		"EOF"))

	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"connect", "status"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Ready: false")
	assert.Contains(t, out.String(), "Run `auto connect`")
}

func TestConnectDocsStayInSync(t *testing.T) {
	t.Parallel()

	assertConnectStateMachineCopy(t, newConnectCmd().Long)

	readmePath := filepath.Join("..", "..", "README.md")
	data, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	readme := string(data)

	assertConnectStateMachineCopy(t, readme)

	koPath := filepath.Join("..", "..", "docs", "README.ko.md")
	koData, err := os.ReadFile(koPath)
	require.NoError(t, err)
	koReadme := string(koData)

	assertConnectStateMachineCopy(t, koReadme)
}

func assertConnectStateMachineCopy(t *testing.T, text string) {
	t.Helper()

	assert.Contains(t, text, "server auth → workspace → OpenAI OAuth")
	assert.Contains(t, text, "auto connect status")
	assert.NotContains(t, text, "detect → configure → verify")
	assert.NotContains(t, text, "감지 → 설정 → 검증")
}
