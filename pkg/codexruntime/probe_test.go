package codexruntime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const catalogProbeTestTimeout = time.Minute

func TestProbeModelCatalogReturnsValidatedOutput(t *testing.T) {
	t.Parallel()

	payload := `{"models":[{"slug":"gpt-5.6-sol","supported_reasoning_levels":[{"effort":"ultra"}]}]}`
	binary := writeCatalogProbe(t, fmt.Sprintf("printf '%%s' '%s'", payload))

	got, err := ProbeModelCatalog(context.Background(), binary, catalogProbeTestTimeout)
	require.NoError(t, err)
	assert.Equal(t, payload, string(got))
}

func TestProbeModelCatalogRejectsOversizedStdout(t *testing.T) {
	t.Parallel()

	binary := writeCatalogProbe(t, fmt.Sprintf("yes x | head -c %d", config.MaxCodexModelCatalogBytes+1))

	_, err := ProbeModelCatalog(context.Background(), binary, catalogProbeTestTimeout)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestProbeModelCatalogEnforcesTimeout(t *testing.T) {
	t.Parallel()

	binary := writeCatalogProbe(t, "sleep 30")
	started := time.Now()

	_, err := ProbeModelCatalog(context.Background(), binary, 50*time.Millisecond)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
	assert.Less(t, time.Since(started), 5*time.Second)
}

func writeCatalogProbe(t *testing.T, command string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "codex-probe")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+command+"\n"), 0755))
	return path
}
