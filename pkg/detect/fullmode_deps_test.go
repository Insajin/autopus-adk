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

func TestFullModeDeps_GeminiUsesGooglePackage(t *testing.T) {
	t.Parallel()

	for _, dep := range FullModeDeps {
		if dep.Name == "gemini" {
			assert.Contains(t, dep.InstallCmd, "@google/gemini-cli")
			return
		}
	}
	t.Fatal("gemini entry not found in FullModeDeps")
}
