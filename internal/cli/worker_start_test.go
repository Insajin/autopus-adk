package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveProvider_PrefersAuthenticatedConfiguredProvider(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	codexDir := filepath.Join(tmpHome, ".codex")
	requireNoError(t, os.MkdirAll(codexDir, 0o755))

	got := resolveProvider([]string{"claude", "codex"})
	assert.Equal(t, "codex", got)
}

func TestResolveProvider_FallsBackToFirstConfiguredWhenNoneAuthenticated(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	got := resolveProvider([]string{"claude", "codex"})
	assert.Equal(t, "claude", got)
}

func TestEffectiveWorkerConcurrency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		requested    int
		expected     int
	}{
		{name: "codex clamps parallel requests", providerName: "codex", requested: 3, expected: 1},
		{name: "codex leaves sequential unchanged", providerName: "codex", requested: 1, expected: 1},
		{name: "other providers keep configured concurrency", providerName: "claude", requested: 3, expected: 3},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := effectiveWorkerConcurrency(tc.providerName, tc.requested); got != tc.expected {
				t.Fatalf("effectiveWorkerConcurrency(%q, %d) = %d, want %d",
					tc.providerName, tc.requested, got, tc.expected)
			}
		})
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
