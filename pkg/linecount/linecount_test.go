package linecount

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func count(t *testing.T, filename, body string) Counts {
	t.Helper()
	counts, err := Source(filename, []byte(body))
	require.NoError(t, err)
	return counts
}

// The physical total is what the limit used to be measured against, so its
// convention has to survive: a final line without a newline still counts, and
// a CRLF pair is one break rather than two.
func TestPhysicalLineConvention(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		body string
		want int
	}{
		{"empty file", "", 0},
		{"single newline", "\n", 1},
		{"no final newline", "a\nb", 2},
		{"final newline", "a\nb\n", 2},
		{"crlf", "a\r\nb\r\n", 2},
		{"crlf without final break", "a\r\nb", 2},
		{"lone carriage return", "a\rb", 1},
		{"trailing blank lines", "a\n\n\n", 3},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, row.want, Physical([]byte(row.body)))
			assert.Equal(t, row.want, count(t, "x.go", row.body).Physical)
		})
	}
}

// An empty file has nothing to lex; the zero result has to come out of the
// public API rather than the lexer being asked for a line that is not there.
func TestEmptyFileCountsZero(t *testing.T) {
	t.Parallel()
	assert.Equal(t, Counts{}, count(t, "x.go", ""))
}

func TestCountedIsPhysicalMinusCommentOnly(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "// a\n// b\npackage main\n")
	assert.Equal(t, Counts{Physical: 3, CommentOnly: 2, Counted: 1}, counts)
}

// The whole point of the change: a file at the limit stays at the limit however
// much prose is wrapped around it.
func TestCommentVolumeDoesNotChangeCountedCode(t *testing.T) {
	t.Parallel()
	var body strings.Builder
	for i := range 300 {
		for range 2 {
			body.WriteString("// filler\n")
		}
		body.WriteString("var x")
		body.WriteString(string(rune('A' + i%26)))
		body.WriteString(" = 1\n")
	}
	counts := count(t, "x.go", body.String())
	assert.Equal(t, 900, counts.Physical)
	assert.Equal(t, 600, counts.CommentOnly)
	assert.Equal(t, 300, counts.Counted)
}

func TestCountedTracksCodeLinesPastTheLimit(t *testing.T) {
	t.Parallel()
	var body strings.Builder
	body.WriteString("/*\n a block comment\n*/\n")
	for range 301 {
		body.WriteString("x := 1\n")
	}
	assert.Equal(t, 301, count(t, "x.go", body.String()).Counted)
}

// A file made only of comments still occupies lines on disk, so Physical has to
// report them even though nothing is counted.
func TestCommentOnlyFileCountsNoCode(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "// a\n/* b\n\n   c */\n// d\n")
	assert.Equal(t, 5, counts.Physical)
	assert.Equal(t, 5, counts.CommentOnly)
	assert.Zero(t, counts.Counted)
}

// A blank line inside a block comment belongs to the comment; a blank line
// outside one is ordinary file structure and stays counted.
func TestBlankLinesFollowTheirSurroundings(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "/*\n\n*/\n\npackage main\n")
	assert.Equal(t, 5, counts.Physical)
	assert.Equal(t, 3, counts.CommentOnly)
	assert.Equal(t, 2, counts.Counted)
}

func TestCodeSharingALineWithACommentIsCounted(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "x := 1 // why\n")
	assert.Zero(t, counts.CommentOnly)
	assert.Equal(t, 1, counts.Counted)
}

// Only the interior of a block comment is comment-only: the lines that also
// carry code at either end are still code.
func TestBlockCommentBoundedByCodeKeepsBothEnds(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "x := 1 /* open\n  interior\n  more\n*/ ; y := 2\n")
	assert.Equal(t, 4, counts.Physical)
	assert.Equal(t, 2, counts.CommentOnly)
	assert.Equal(t, 2, counts.Counted)
}

func TestFileMatchesInMemoryBytes(t *testing.T) {
	t.Parallel()
	body := "// doc\npackage main\n\nfunc main() {} // run\n"
	path := filepath.Join(t.TempDir(), "main.go")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	fromDisk, err := File(path)
	require.NoError(t, err)
	assert.Equal(t, count(t, path, body), fromDisk)
	assert.Equal(t, Counts{Physical: 4, CommentOnly: 1, Counted: 3}, fromDisk)
}

func TestFileReportsUnreadablePath(t *testing.T) {
	t.Parallel()
	_, err := File(filepath.Join(t.TempDir(), "absent.go"))
	require.Error(t, err)
}

// An unrecognised extension is measured but not analysed, so it can never be
// reported as smaller than its physical size.
func TestUnknownExtensionCountsEveryLine(t *testing.T) {
	t.Parallel()
	counts := count(t, "notes.txt", "// looks like code\n# and like more\n")
	assert.Equal(t, Counts{Physical: 2, Counted: 2}, counts)
}

// Line-based readers cap a line at 64 KiB by default; a minified bundle or a
// generated table blows straight past that, and silently truncating it would
// misreport the file.
func TestVeryLongSingleLineIsCounted(t *testing.T) {
	t.Parallel()
	body := "const blob = \"" + strings.Repeat("payload", 30_000) + "\";\n// tail\n"
	require.Greater(t, len(body), 64<<10)
	counts := count(t, "bundle.js", body)
	assert.Equal(t, 2, counts.Physical)
	assert.Equal(t, 1, counts.CommentOnly)
	assert.Equal(t, 1, counts.Counted)
}

// Bytes that are not valid UTF-8 get substituted during lexing, which changes
// byte offsets but not line breaks. The counts must stay anchored to the file.
func TestMalformedBytesDoNotBreakCounting(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "package \xff\xfe\x00main\n// a\nx := 1\n")
	assert.Equal(t, 3, counts.Physical)
	assert.Equal(t, 1, counts.CommentOnly)
	assert.Equal(t, 2, counts.Counted)
}

// An unterminated block comment is not valid source. Whatever the lexer makes
// of it, the file may not shrink below the lines it really has.
func TestUnterminatedCommentNeverUndercounts(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.go", "package main\n/* never closed\nstill going\n")
	assert.Equal(t, 3, counts.Physical)
	assert.LessOrEqual(t, counts.CommentOnly, 2)
	assert.Equal(t, counts.Physical-counts.CommentOnly, counts.Counted)
}
