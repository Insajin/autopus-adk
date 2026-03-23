package search_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/search"
)

func TestHashFile_Basic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "line 1\nline 2\nline 3\n"
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	lines, err := search.HashFile(path)
	require.NoError(t, err)

	require.Len(t, lines, 3)
	assert.Equal(t, 1, lines[0].LineNumber)
	assert.Equal(t, "line 1", lines[0].Content)
	assert.NotEmpty(t, lines[0].Hash)
}

func TestHashFile_OutputFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "format.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello\nworld\n"), 0o644))

	lines, err := search.HashFile(path)
	require.NoError(t, err)

	// 형식: LINE#HASH
	for _, l := range lines {
		formatted := fmt.Sprintf("%d#%s", l.LineNumber, l.Hash)
		assert.Contains(t, formatted, "#")
		assert.NotEmpty(t, l.Hash)
	}
}

func TestHashFile_Deterministic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "same.txt")
	require.NoError(t, os.WriteFile(path, []byte("same content\n"), 0o644))

	lines1, err := search.HashFile(path)
	require.NoError(t, err)

	lines2, err := search.HashFile(path)
	require.NoError(t, err)

	// 동일 내용은 동일 해시
	require.Len(t, lines1, len(lines2))
	for i := range lines1 {
		assert.Equal(t, lines1[i].Hash, lines2[i].Hash)
	}
}

func TestHashFile_DifferentLines(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "diff.txt")
	require.NoError(t, os.WriteFile(path, []byte("line a\nline b\n"), 0o644))

	lines, err := search.HashFile(path)
	require.NoError(t, err)
	require.Len(t, lines, 2)

	// 다른 내용은 다른 해시
	assert.NotEqual(t, lines[0].Hash, lines[1].Hash)
}

func TestHashFile_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))

	lines, err := search.HashFile(path)
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestHashFile_NonExistentFile(t *testing.T) {
	t.Parallel()

	_, err := search.HashFile("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestHashFile_LineNumbering(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := "a\nb\nc\nd\ne\n"
	path := filepath.Join(dir, "numbered.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	lines, err := search.HashFile(path)
	require.NoError(t, err)
	require.Len(t, lines, 5)

	for i, l := range lines {
		assert.Equal(t, i+1, l.LineNumber)
	}
}

func TestFormatHashLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line search.HashLine
		want string
	}{
		{
			name: "basic format",
			line: search.HashLine{LineNumber: 1, Hash: "abc123"},
			want: "1#abc123",
		},
		{
			name: "large line number",
			line: search.HashLine{LineNumber: 9999, Hash: "def456"},
			want: "9999#def456",
		},
		{
			name: "empty hash",
			line: search.HashLine{LineNumber: 5, Hash: ""},
			want: "5#",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, search.FormatHashLine(tc.line))
		})
	}
}
