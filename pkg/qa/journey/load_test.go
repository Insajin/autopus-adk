package journey

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFileDefaultsCWDAndReportsErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pack.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`id: x
lanes: [fast]
adapter:
  id: go-test
command:
  run: go test ./...
checks:
  - id: unit
    type: unit_test
`), 0o644))

	pack, err := LoadFile(path)

	require.NoError(t, err)
	assert.Equal(t, ".", pack.Command.CWD)

	_, err = LoadFile(filepath.Join(dir, "missing.yaml"))
	require.Error(t, err)

	require.NoError(t, os.WriteFile(path, []byte("id: [unterminated\n"), 0o644))
	_, err = LoadFile(path)
	require.Error(t, err)
}

func TestHasLaneAllowsEmptyLaneAndTrimsCase(t *testing.T) {
	t.Parallel()

	pack := Pack{Lanes: []string{" Fast "}}

	assert.True(t, HasLane(pack, ""))
	assert.True(t, HasLane(pack, "fast"))
	assert.False(t, HasLane(pack, "release"))
}
