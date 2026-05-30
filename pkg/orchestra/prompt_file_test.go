package orchestra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPanePromptText_WritesMarkdownAndReturnsFileInstruction(t *testing.T) {
	t.Parallel()
	workingDir := t.TempDir()
	cfg := OrchestraConfig{WorkingDir: workingDir}
	provider := ProviderConfig{Name: "codex", Binary: "codex"}

	instruction, path, responsePath := panePromptText(cfg, provider, 2, "full prompt body")
	defer cleanupPromptFiles([]string{path, responsePath})

	require.NotEmpty(t, path)
	require.NotEmpty(t, responsePath)
	assert.Contains(t, filepath.ToSlash(path), ".autopus/orchestra/prompts/codex-round-2-")
	assert.Contains(t, filepath.ToSlash(responsePath), ".autopus/orchestra/responses/codex-round-2-")
	assert.Contains(t, instruction, "@"+path)
	assert.Contains(t, instruction, responsePath)
	assert.Contains(t, instruction, responseBeginMarker)
	assert.Contains(t, instruction, responseEndMarker)
	assert.Contains(t, instruction, "Treat the prompt file contents as the full user request.")

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "full prompt body", string(content))

	responseContent, err := os.ReadFile(responsePath)
	require.NoError(t, err)
	assert.Contains(t, string(responseContent), responseBeginMarker)
	assert.Contains(t, string(responseContent), responseEndMarker)
}

func TestPanePromptText_EmptyPromptDoesNotCreateFile(t *testing.T) {
	t.Parallel()
	instruction, path, responsePath := panePromptText(OrchestraConfig{WorkingDir: t.TempDir()}, ProviderConfig{Name: "claude"}, 1, "   ")
	assert.Equal(t, "   ", instruction)
	assert.Empty(t, path)
	assert.Empty(t, responsePath)
}

func TestReadResponseFile_ExtractsMarkedMarkdown(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "response.md")
	content := "# notes\n" + responseBeginMarker + "\nfinal **answer**\n" + responseEndMarker + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	output, ok := readResponseFile(path)
	require.True(t, ok)
	assert.Equal(t, "final **answer**", output)
}

func TestReadResponseFile_RejectsMissingOrEmptyMarkers(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "response.md")
	require.NoError(t, os.WriteFile(path, []byte(responseBeginMarker+"\n\n"+responseEndMarker+"\n"), 0o600))

	output, ok := readResponseFile(path)
	assert.False(t, ok)
	assert.Empty(t, output)

	require.NoError(t, os.WriteFile(path, []byte("plain answer"), 0o600))
	output, ok = readResponseFile(path)
	assert.False(t, ok)
	assert.Empty(t, output)
}
