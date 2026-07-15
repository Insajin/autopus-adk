package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func TestRequireCompleteGPTReviewDocuments_IsGPTOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []orchestra.ProviderConfig
		want      bool
	}{
		{name: "codex", providers: []orchestra.ProviderConfig{{Name: "codex"}}, want: true},
		{name: "gpt aliases", providers: []orchestra.ProviderConfig{{Name: "gpt"}, {Name: "openai"}}, want: true},
		{name: "codex binary", providers: []orchestra.ProviderConfig{{Binary: "/usr/local/bin/codex"}}, want: true},
		{name: "mixed", providers: []orchestra.ProviderConfig{{Name: "codex"}, {Name: "claude"}}},
		{name: "gemini", providers: []orchestra.ProviderConfig{{Name: "gemini"}}},
		{name: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, requireCompleteGPTReviewDocuments(tt.providers))
		})
	}
}

func TestResolveSpecReviewContextScope_DerivesNestedProjectRoot(t *testing.T) {
	t.Parallel()

	projectRoot := filepath.Join(t.TempDir(), "nested-project")
	specDir := filepath.Join(projectRoot, ".autopus", "specs", "SPEC-SCOPE-001")
	require.NoError(t, os.MkdirAll(specDir, 0o700))

	scope, err := resolveSpecReviewContextScope(specDir)

	require.NoError(t, err)
	canonicalRoot, err := filepath.EvalSymlinks(projectRoot)
	require.NoError(t, err)
	assert.Equal(t, canonicalRoot, scope.projectRoot)
	assert.Equal(t, ".autopus/specs/SPEC-SCOPE-001", scope.specDir)
}

func TestResolveSpecReviewContextScope_RejectsNonSpecDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "docs", "SPEC-SCOPE-001")
	require.NoError(t, os.MkdirAll(dir, 0o700))

	_, err := resolveSpecReviewContextScope(dir)

	require.Error(t, err)
	assert.ErrorContains(t, err, "outside .autopus/specs")
}

func TestPrepareSpecReviewContextDelivery_MixedProvidersPreserveLegacyPath(t *testing.T) {
	t.Parallel()

	delivery, err := prepareSpecReviewContextDelivery(
		"not-a-resolved-spec-directory",
		[]orchestra.ProviderConfig{{Name: "codex"}, {Name: "claude"}},
		specReviewOptions{requiredDocuments: []string{"../ignored-on-legacy-path"}},
	)

	require.NoError(t, err)
	assert.Nil(t, delivery)
}
