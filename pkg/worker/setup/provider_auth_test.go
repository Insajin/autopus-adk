package setup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckProviderAuth_UnknownProvider(t *testing.T) {
	t.Parallel()

	authenticated, guide := CheckProviderAuth("unknown-provider")
	assert.False(t, authenticated)
	assert.Contains(t, guide, "Unknown provider")
}

func TestCheckProviderAuth_Claude_NoCredentials(t *testing.T) {
	// Use a temp dir as HOME so credentials.json won't exist.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	authenticated, guide := CheckProviderAuth("claude")
	assert.False(t, authenticated)
	assert.Contains(t, guide, "claude login")
}

func TestCheckProviderAuth_Claude_WithCredentials(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Create the credentials file.
	credDir := filepath.Join(tmp, ".claude")
	err := os.MkdirAll(credDir, 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(credDir, "credentials.json"), []byte("{}"), 0600)
	assert.NoError(t, err)

	authenticated, guide := CheckProviderAuth("claude")
	assert.True(t, authenticated)
	assert.Empty(t, guide)
}

func TestCheckProviderAuth_Codex_WithEnvVar(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	authenticated, guide := CheckProviderAuth("codex")
	assert.True(t, authenticated)
	assert.Empty(t, guide)
}

func TestCheckProviderAuth_Gemini_WithEnvVar(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "test-google-key")

	authenticated, guide := CheckProviderAuth("gemini")
	assert.True(t, authenticated)
	assert.Empty(t, guide)
}

func TestCheckProviderAuth_Claude_GuideContainsURL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_, guide := CheckProviderAuth("claude")
	assert.Contains(t, guide, "https://claude.ai")
	assert.Contains(t, guide, "1.")
	assert.Contains(t, guide, "2.")
	assert.Contains(t, guide, "3.")
}

func TestCheckProviderAuth_Codex_GuideContainsURL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("OPENAI_API_KEY", "")

	_, guide := CheckProviderAuth("codex")
	assert.Contains(t, guide, "https://platform.openai.com")
	assert.Contains(t, guide, "export OPENAI_API_KEY")
	assert.Contains(t, guide, "1.")
	assert.Contains(t, guide, "2.")
}

func TestCheckProviderAuth_Gemini_GuideContainsURL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GOOGLE_API_KEY", "")

	_, guide := CheckProviderAuth("gemini")
	assert.Contains(t, guide, "https://aistudio.google.com")
	assert.Contains(t, guide, "export GOOGLE_API_KEY")
	assert.Contains(t, guide, "1.")
	assert.Contains(t, guide, "2.")
}

func TestCheckProviderAuth_Opencode_GuideContainsURL(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	_, guide := CheckProviderAuth("opencode")
	assert.Contains(t, guide, "https://platform.openai.com")
	assert.Contains(t, guide, "export OPENAI_API_KEY")
	assert.Contains(t, guide, "1.")
	assert.Contains(t, guide, "2.")
}

func TestCheckProviderAuth_Opencode_WithEnvVar(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-key")

	authenticated, guide := CheckProviderAuth("opencode")
	assert.True(t, authenticated)
	assert.Empty(t, guide)
}

func TestCheckProviderAuth_Codex_WithDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("OPENAI_API_KEY", "")

	// Create .codex directory
	codexDir := filepath.Join(tmp, ".codex")
	require.NoError(t, os.MkdirAll(codexDir, 0755))

	authenticated, guide := CheckProviderAuth("codex")
	assert.True(t, authenticated)
	assert.Empty(t, guide)
}

func TestCheckProviderAuth_Codex_NoAuth(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("OPENAI_API_KEY", "")

	authenticated, guide := CheckProviderAuth("codex")
	assert.False(t, authenticated)
	assert.Contains(t, guide, "codex login")
}

func TestCheckProviderAuth_Gemini_WithDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GOOGLE_API_KEY", "")

	// Create .config/gemini directory
	geminiDir := filepath.Join(tmp, ".config", "gemini")
	require.NoError(t, os.MkdirAll(geminiDir, 0755))

	authenticated, guide := CheckProviderAuth("gemini")
	assert.True(t, authenticated)
	assert.Empty(t, guide)
}

func TestCheckProviderAuth_Gemini_NoAuth(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("GOOGLE_API_KEY", "")

	authenticated, guide := CheckProviderAuth("gemini")
	assert.False(t, authenticated)
	assert.Contains(t, guide, "gemini login")
}

func TestCheckProviderAuth_Opencode_NoAuth(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")

	authenticated, guide := CheckProviderAuth("opencode")
	assert.False(t, authenticated)
	assert.Contains(t, guide, "export OPENAI_API_KEY")
}

func TestFileExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string) string
		expected bool
	}{
		{
			name: "existing file",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "test.txt")
				require.NoError(t, os.WriteFile(p, []byte("content"), 0600))
				return p
			},
			expected: true,
		},
		{
			name: "nonexistent path",
			setup: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "nope.txt")
			},
			expected: false,
		},
		{
			name: "directory instead of file",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "subdir")
				require.NoError(t, os.MkdirAll(p, 0755))
				return p
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := tt.setup(t, dir)
			assert.Equal(t, tt.expected, fileExists(path))
		})
	}
}

func TestDirExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string) string
		expected bool
	}{
		{
			name: "existing directory",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "subdir")
				require.NoError(t, os.MkdirAll(p, 0755))
				return p
			},
			expected: true,
		},
		{
			name: "nonexistent path",
			setup: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "nope")
			},
			expected: false,
		},
		{
			name: "file instead of directory",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "file.txt")
				require.NoError(t, os.WriteFile(p, []byte("content"), 0600))
				return p
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := tt.setup(t, dir)
			assert.Equal(t, tt.expected, dirExists(path))
		})
	}
}
