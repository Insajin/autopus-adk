package adapter

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnvironWithToolPathAppendsWellKnownDirs is the S1 oracle for REQ-001:
// inherited PATH entries resolve before well-known tool directories.
func TestEnvironWithToolPathAppendsWellKnownDirs(t *testing.T) {
	env := EnvironWithToolPath([]string{
		"FOO=bar",
		"PATH=/usr/bin:/bin",
	})

	assert.Contains(t, env, "FOO=bar")
	pathValue := envValue(env, "PATH")
	require.NotEmpty(t, pathValue)

	parts := strings.Split(pathValue, string(os.PathListSeparator))
	assert.Equal(t, "/usr/bin", parts[0])
	assert.Equal(t, "/bin", parts[1])

	homebrewIdx := indexOf(parts, "/opt/homebrew/bin")
	require.GreaterOrEqual(t, homebrewIdx, 2)
	assert.Less(t, indexOf(parts, "/usr/bin"), homebrewIdx)
	assert.Contains(t, parts, "/usr/local/bin")
}

// TestEnvironWithToolPathDedupesSharedDir is the S2 oracle for REQ-003: a
// directory present in both the inherited PATH and wellKnownDirs keeps only
// its inherited-PATH occurrence.
func TestEnvironWithToolPathDedupesSharedDir(t *testing.T) {
	env := EnvironWithToolPath([]string{
		"PATH=/usr/local/bin:/usr/bin",
	})

	pathValue := envValue(env, "PATH")
	parts := strings.Split(pathValue, string(os.PathListSeparator))

	assert.Equal(t, 1, strings.Count(pathValue, "/usr/local/bin"))
	assert.Equal(t, 0, indexOf(parts, "/usr/local/bin"))
}

// TestEnvironWithToolPathKeepsUnlistedWellKnownDirsSearchable is the S3
// oracle for REQ-002: a well-known directory absent from the inherited PATH
// is still appended so subprocesses can find tools installed there.
func TestEnvironWithToolPathKeepsUnlistedWellKnownDirsSearchable(t *testing.T) {
	env := EnvironWithToolPath([]string{
		"PATH=/usr/bin:/bin",
	})

	pathValue := envValue(env, "PATH")
	parts := strings.Split(pathValue, string(os.PathListSeparator))

	homebrewIdx := indexOf(parts, "/opt/homebrew/bin")
	require.GreaterOrEqual(t, homebrewIdx, 0, "well-known dir must still be present")
	assert.Less(t, indexOf(parts, "/usr/bin"), homebrewIdx)
	assert.Less(t, indexOf(parts, "/bin"), homebrewIdx)
}

func indexOf(parts []string, target string) int {
	for i, part := range parts {
		if part == target {
			return i
		}
	}
	return -1
}

func TestEnvironWithToolPathUsesLastInputPathAndDedupes(t *testing.T) {
	env := EnvironWithToolPath([]string{
		"PATH=/first",
		"PATH=/usr/local/bin:/custom",
	})

	pathValue := envValue(env, "PATH")
	assert.Contains(t, strings.Split(pathValue, string(os.PathListSeparator)), "/custom")
	assert.NotContains(t, strings.Split(pathValue, string(os.PathListSeparator)), "/first")
	assert.Equal(t, 1, strings.Count(pathValue, "/usr/local/bin"))
	assert.Equal(t, 1, strings.Count(strings.Join(env, "\n"), "PATH="))
}
