package claude_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContextProfiles_GeneratedDetails_MatchS8Matrix(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, err := claude.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("context-profile"))
	require.NoError(t, err)

	tests := []struct {
		route    string
		required []string
		excluded []string
	}{
		{
			route: "plan",
			required: []string{
				"Required: core workspace policy, architecture, and relevant SPEC evidence.",
				"Conditional: signatures and learnings only when explicitly declared",
				"Excluded by default: scenarios and canary.",
			},
		},
		{
			route:    "test",
			required: []string{"Required: core workspace policy and scenarios.", "Excluded: canary, signatures, and unrelated learnings."},
			excluded: []string{"Required: core workspace policy and canary."},
		},
		{
			route:    "canary",
			required: []string{"Required: core workspace policy, canary, and the declared canary command.", "Excluded: scenarios, signatures, and unrelated learnings."},
			excluded: []string{"Required: core workspace policy and scenarios."},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.route, func(t *testing.T) {
			detail := readClaudeSurface(t, root, filepath.Join(".claude", "skills", "autopus", "auto-"+tt.route+".md"))
			profile := contextProfileSection(t, detail)
			for _, token := range tt.required {
				assert.Contains(t, profile, token)
			}
			for _, token := range tt.excluded {
				assert.NotContains(t, profile, token)
			}
		})
	}
}

func contextProfileSection(t *testing.T, detail string) string {
	t.Helper()
	const startMarker = "## Context Profile"
	const endMarker = "## Contract"
	start := strings.Index(detail, startMarker)
	end := strings.Index(detail, endMarker)
	require.GreaterOrEqual(t, start, 0)
	require.Greater(t, end, start)
	return detail[start:end]
}
