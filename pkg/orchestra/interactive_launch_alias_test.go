package orchestra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInteractiveLaunchArgs_GeminiArtifactAliasesSuppressPrintArgs(t *testing.T) {
	t.Parallel()

	geminiProvider := func(name, binary string) ProviderConfig {
		return ProviderConfig{
			Name: name, Binary: binary, Args: []string{"--print", ""},
			PromptViaArgs: true, InteractiveInput: "stdin",
		}
	}
	tests := []struct {
		name     string
		provider ProviderConfig
		want     []string
		wantCmd  string
	}{
		{name: "gemini", provider: geminiProvider("gemini", "agy"), wantCmd: "agy --dangerously-skip-permissions --prompt-interactive 'test prompt'"},
		{name: "antigravity", provider: geminiProvider("antigravity", "agy"), wantCmd: "agy --dangerously-skip-permissions --prompt-interactive 'test prompt'"},
		{name: "antigravity cli", provider: geminiProvider("antigravity-cli", "agy"), wantCmd: "agy --dangerously-skip-permissions --prompt-interactive 'test prompt'"},
		{name: "gemini cli", provider: geminiProvider("gemini-cli", "agy"), wantCmd: "agy --dangerously-skip-permissions --prompt-interactive 'test prompt'"},
		{name: "agy", provider: geminiProvider("agy", "agy"), wantCmd: "agy --dangerously-skip-permissions --prompt-interactive 'test prompt'"},
		{
			name:     "agy path",
			provider: geminiProvider("gemini-cli", "/opt/autopus/bin/agy"),
			wantCmd:  "/opt/autopus/bin/agy --dangerously-skip-permissions --prompt-interactive 'test prompt'",
		},
		{
			name:     "custom provider keeps its print args",
			provider: geminiProvider("custom", "agy"),
			want:     []string{"--print", ""},
			wantCmd:  "agy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, interactiveLaunchArgs(tt.provider))
			assert.Equal(t, tt.wantCmd, buildInteractiveLaunchCmd(tt.provider, "test prompt"))
		})
	}
}

func TestPromptDeliveredAtLaunch_GeminiArtifactAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ProviderConfig
		want     bool
	}{
		{name: "gemini", provider: ProviderConfig{Name: "gemini", Binary: "agy"}, want: true},
		{name: "antigravity", provider: ProviderConfig{Name: "antigravity", Binary: "agy"}, want: true},
		{name: "antigravity cli", provider: ProviderConfig{Name: "antigravity-cli", Binary: "agy"}, want: true},
		{name: "gemini cli", provider: ProviderConfig{Name: "gemini-cli", Binary: "agy"}, want: true},
		{name: "agy", provider: ProviderConfig{Name: "agy", Binary: "agy"}, want: true},
		{name: "custom agy binary", provider: ProviderConfig{Name: "custom", Binary: "agy"}, want: false},
		{name: "gemini other binary", provider: ProviderConfig{Name: "gemini", Binary: "gemini"}, want: false},
		{
			name:     "custom args mode",
			provider: ProviderConfig{Name: "custom", Binary: "custom", InteractiveInput: "args"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, promptDeliveredAtLaunch(tt.provider))
		})
	}
}

func TestResolveHookProviders_GeminiArtifactAliasesOwnCanonicalHook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		provider             ProviderConfig
		wantGeminiHook       bool
		wantProviderMapEntry bool
	}{
		{
			name: "gemini", provider: ProviderConfig{Name: "gemini", Binary: "agy"},
			wantProviderMapEntry: true,
		},
		{name: "antigravity", provider: ProviderConfig{Name: "antigravity", Binary: "agy"}},
		{name: "antigravity cli", provider: ProviderConfig{Name: "antigravity-cli", Binary: "agy"}},
		{name: "gemini cli", provider: ProviderConfig{Name: "gemini-cli", Binary: "agy"}},
		{name: "agy", provider: ProviderConfig{Name: "agy", Binary: "agy"}},
		{
			name:           "custom agy binary",
			provider:       ProviderConfig{Name: "custom", Binary: "agy"},
			wantGeminiHook: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveHookProviders([]ProviderConfig{tt.provider})
			assert.Equal(t, tt.wantGeminiHook, got["gemini"])
			_, hasProviderEntry := got[tt.provider.Name]
			assert.Equal(t, tt.wantProviderMapEntry, hasProviderEntry,
				"hook capability must be owned by the canonical artifact identity")
		})
	}
}
