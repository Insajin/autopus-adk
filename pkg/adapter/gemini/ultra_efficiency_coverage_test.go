package gemini

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUltraEfficiencyCoverage_UpdateRejectsCorruptCurrentAndLegacyManifests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		platform string
		want     string
	}{
		{name: "current", platform: adapterName, want: "매니페스트 로드 실패"},
		{name: "legacy", platform: legacyAdapterName, want: "legacy 매니페스트 로드 실패"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			manifestDir := filepath.Join(root, ".autopus")
			require.NoError(t, os.MkdirAll(manifestDir, 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(manifestDir, test.platform+"-manifest.json"), []byte("{invalid"), 0o600,
			))

			_, err := NewWithRoot(root).Update(
				context.Background(), config.DefaultFullConfig("coverage-project"),
			)
			assert.ErrorContains(t, err, test.want)
		})
	}
}

func TestUltraEfficiencyCoverage_NonSkillFileIsNotAnAutoRouteTarget(t *testing.T) {
	t.Parallel()
	assert.False(t, isAutoRouteSkillTarget(filepath.Join(".gemini", "skills", "autopus", "auto-plan", "README.md")))
}
