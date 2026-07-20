package orchestra

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func typedJudgeProvider(t *testing.T, name string) ProviderConfig {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("typed judge fixture requires a POSIX shell")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "typed-judge")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '{\"recommendation\":\"typed judge fixture\"}'\n"
	require.NoError(t, os.WriteFile(binary, []byte(script), 0o700))
	return ProviderConfig{Name: name, Binary: binary, ModelFamily: "judge-fixture"}
}
