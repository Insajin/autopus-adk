package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		runGitCommand(t, dir, args...)
	}
}

func runGitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
	return string(out)
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func writeGoFileWithComments(t *testing.T, dir, name string, commentLines int) string {
	t.Helper()

	var sb strings.Builder
	sb.WriteString("package dummy\n")
	for i := 0; i < commentLines; i++ {
		sb.WriteString("// line\n")
	}
	return writeTestFile(t, dir, name, sb.String())
}
