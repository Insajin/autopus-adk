package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/stretchr/testify/require"
)

func TestRunStructuredSpecReviewOrchestra_CleansOwnedHookSession(t *testing.T) {
	const sid = "orch-test-structured-owned-hook"
	sessionDir := filepath.Join(os.TempDir(), "autopus", sid)
	require.NoError(t, os.RemoveAll(sessionDir))
	defer func() { _ = os.RemoveAll(sessionDir) }()

	backend := &fakeStructuredReviewBackend{
		name: "pane",
		outputs: map[string]orchestra.ProviderResponse{
			"claude": {Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		},
	}
	origFactory := specReviewBackendFactory
	specReviewBackendFactory = func(orchestra.OrchestraConfig) orchestra.ExecutionBackend { return backend }
	defer func() { specReviewBackendFactory = origFactory }()

	result, err := runStructuredSpecReviewOrchestra(context.Background(), orchestra.OrchestraConfig{
		Providers:      []orchestra.ProviderConfig{{Name: "claude", Binary: "claude"}},
		Prompt:         "Review this SPEC",
		TimeoutSeconds: 10,
		HookMode:       true,
		SessionID:      sid,
	})
	require.NoError(t, err)
	require.Len(t, result.Responses, 1)
	require.NoDirExists(t, sessionDir,
		"structured review should own and clean the shared hook session after all parallel providers finish")
}
