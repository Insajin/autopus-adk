package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDesktopAuthImportCmd_DelegatesToRuntimeHelper(t *testing.T) {
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.txt")
	stdinPath := filepath.Join(tempDir, "stdin.json")
	helperBody := fmt.Sprintf(`printf '%%s\n' "$@" > %q
cat > %q
cat <<'EOF'
{
  "ok": true,
  "backend_url": "https://api.autopus.co",
  "workspace_id": "ws-123"
}
EOF`, argsPath, stdinPath)
	t.Setenv(runtimeHelperOverrideEnv, writeRuntimeHelperScript(t, helperBody))

	cmd := NewRootCmd()
	var out bytes.Buffer
	payload := `{"backend_url":"https://api.autopus.co","workspace_id":"ws-123","access_token":"jwt-token"}`
	cmd.SetIn(strings.NewReader(payload))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"desktop", "auth", "import"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), `"ok": true`)
	assert.Contains(t, out.String(), `"workspace_id": "ws-123"`)

	argsData, err := os.ReadFile(argsPath)
	require.NoError(t, err)
	assert.Equal(t, "desktop\nauth\nimport\n", string(argsData))

	stdinData, err := os.ReadFile(stdinPath)
	require.NoError(t, err)
	assert.Equal(t, payload, string(stdinData))
}

func TestDesktopAuthImportCmd_PropagatesHelperFailure(t *testing.T) {
	helperPath := writeRuntimeHelperScript(t, "echo 'helper failed' >&2\nexit 17\n")

	_, err := executeRootWithEnv(
		t,
		"",
		[]string{runtimeHelperOverrideEnv + "=" + helperPath},
		"desktop",
		"auth",
		"import",
	)
	require.Error(t, err)
	assert.ErrorContains(t, err, "exit status 17")
	assert.ErrorContains(t, err, "helper failed")
}
