package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestInferReviewRiskTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []string
		want  reviewRiskTier
	}{
		{name: "docs only", files: []string{"README.md", ".autopus/specs/SPEC-X/spec.md"}, want: reviewRiskTierLow},
		{name: "ordinary source", files: []string{"pkg/widget/widget.go"}, want: reviewRiskTierMedium},
		{name: "service boundary", files: []string{"internal/services/reporting.go"}, want: reviewRiskTierHigh},
		{name: "auth boundary", files: []string{"internal/services/auth_session.go"}, want: reviewRiskTierCritical},
		{name: "migration boundary", files: []string{"backend/migrations/000447_add_locale.up.sql"}, want: reviewRiskTierCritical},
		{name: "large source fanout", files: []string{"a.go", "b.go", "c.go", "d.go", "e.go"}, want: reviewRiskTierHigh},
		{name: "protobuf contract", files: []string{"proto/public/service.proto"}, want: reviewRiskTierHigh},
		{name: "openapi contract", files: []string{"contracts/openapi.yaml"}, want: reviewRiskTierHigh},
		{name: "swagger contract", files: []string{"contracts/swagger.json"}, want: reviewRiskTierHigh},
		{name: "graphql contract", files: []string{"schema/public.graphql"}, want: reviewRiskTierHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, inferReviewRiskTier(tt.files))
		})
	}
}

func TestDiscoverChangedFilesForRiskTier_IncludesStagedUnstagedAndUntracked(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	runGit("init", "-q")
	for _, name := range []string{"staged.go", "unstaged.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package sample\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runGit("add", ".")
	runGit("commit", "-qm", "base")
	if err := os.WriteFile(filepath.Join(dir, "staged.go"), []byte("package sample\n// staged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit("add", "staged.go")
	if err := os.WriteFile(filepath.Join(dir, "unstaged.go"), []byte("package sample\n// unstaged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.go"), []byte("package sample\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := discoverChangedFilesForRiskTierIn(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"staged.go", "unstaged.go", "untracked.go"}, got)
}

func TestApplyReviewRiskTierProviders(t *testing.T) {
	t.Parallel()

	providers := []orchestra.ProviderConfig{
		{Name: "claude", Binary: "claude"},
		{Name: "codex", Binary: "codex"},
		{Name: "gemini", Binary: "agy"},
	}

	low, degraded := applyReviewRiskTierProviders(providers, reviewRiskTierLow)
	require.False(t, degraded)
	require.Len(t, low, 1)
	assert.Equal(t, "claude", low[0].Name)

	critical, degraded := applyReviewRiskTierProviders(providers[:1], reviewRiskTierCritical)
	require.True(t, degraded)
	require.Len(t, critical, 1)
	assert.Equal(t, "claude", critical[0].Name)

	high, degraded := applyReviewRiskTierProviders(providers, reviewRiskTierHigh)
	require.False(t, degraded)
	assert.Len(t, high, 3)
}

func TestRunSpecReview_MultiFallsBackToSingleProvider(t *testing.T) {
	dir := t.TempDir()
	specDir := scaffoldReviewSpec(t, dir, "SPEC-REVIEW-SINGLE-FALLBACK-001")
	setFakeProviderOnPath(t, dir, "claude")
	t.Setenv("PATH", filepath.Join(dir, "bin"))

	cfg := config.DefaultFullConfig("single-provider-review")
	cfg.Spec.ReviewGate.Providers = []string{"claude"}
	cfg.Orchestra.Providers = map[string]config.ProviderEntry{
		"claude": {
			Binary: "claude",
			Args:   []string{"--print"},
		},
	}
	cfg.Orchestra.Commands = map[string]config.CommandEntry{
		"review": {
			Strategy:  "debate",
			Providers: []string{"claude"},
		},
	}
	require.NoError(t, config.Save(dir, cfg))

	origWD, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(origWD) }()
	require.NoError(t, os.Chdir(dir))

	var capturedProviderCount int
	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		capturedProviderCount = len(cfg.Providers)
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{{
			Provider: "claude",
			Output:   `{"verdict":"PASS","summary":"ok","findings":[]}`,
		}}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	ctx := withGlobalFlags(context.Background(), globalFlags{MultiMode: true})
	require.NoError(t, runSpecReview(ctx, "SPEC-REVIEW-SINGLE-FALLBACK-001", "consensus", 10))
	assert.Equal(t, 1, capturedProviderCount)

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assert.Equal(t, "approved", doc.Status)
}
