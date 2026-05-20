package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFullModeDeps_GitAndGHAreRequired(t *testing.T) {
	t.Parallel()

	required := map[string]bool{
		"git": false,
		"gh":  false,
	}
	for _, dep := range FullModeDeps {
		if _, ok := required[dep.Name]; ok {
			required[dep.Name] = dep.Required
		}
	}

	assert.True(t, required["git"], "git must stay required for install-time bootstrap")
	assert.True(t, required["gh"], "gh must stay required for beginner-friendly GitHub workflows")
}

func TestFullModeDeps_AntigravityUsesAgyCLI(t *testing.T) {
	t.Parallel()

	for _, dep := range FullModeDeps {
		if dep.Name == "antigravity" {
			assert.Equal(t, "agy", dep.Binary)
			assert.Contains(t, dep.Description, "Antigravity CLI")
			assert.True(t, dep.Required, "Antigravity CLI must be installed as a required bootstrap tool")
			assert.True(t, dep.InstallViaShell, "Antigravity CLI installer uses a shell pipeline")
			assert.Contains(t, dep.InstallCmd, "https://antigravity.google/cli/install")
			assert.Empty(t, dep.DependsOn, "Antigravity CLI installer must not be blocked on node")
			return
		}
	}
	t.Fatal("antigravity entry not found in FullModeDeps")
}
