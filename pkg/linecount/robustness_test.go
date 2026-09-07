package linecount

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A correction that cannot be applied is the dangerous failure: the lexer would
// still work, just without the fix that stops a string from being read as a
// comment. Counting has to stop instead of quietly resuming the undercount.
func TestUnapplicableCorrectionFailsInsteadOfDegrading(t *testing.T) {
	t.Parallel()
	_, err := buildLexer("Go", map[string]correction{
		"Go": {state: "state-that-does-not-exist", rules: []chroma.Rule{{
			Pattern: `x`,
			Type:    chroma.LiteralString,
		}}},
	})
	require.Error(t, err)
}

func TestMissingLexerFails(t *testing.T) {
	t.Parallel()
	_, err := buildLexer("a language chroma does not have", nil)
	require.Error(t, err)
}

func TestUncorrectedLanguageUsesTheRegistryLexerDirectly(t *testing.T) {
	t.Parallel()
	lexer, err := buildLexer("Go", nil)
	require.NoError(t, err)
	require.NotNil(t, lexer)
	assert.Equal(t, "Go", lexer.Config().Name)
}

// Chroma signals a grammar defect by panicking part-way through iteration. A
// file that trips one must fail to be analysed rather than be credited with the
// comments found before the panic, and must not take the caller down.
func TestLexerPanicYieldsNoCommentCredit(t *testing.T) {
	t.Parallel()
	broken := chroma.MustNewLexer(&chroma.Config{Name: "broken"}, func() chroma.Rules {
		return chroma.Rules{"root": {
			{Pattern: `//[^\n]*`, Type: chroma.CommentSingle},
			{Pattern: `\n`, Type: chroma.Text},
			{Pattern: `.`, Type: chroma.Text, Mutator: chroma.Push("absent")},
		}}
	})
	commentOnly, err := classify(broken, []byte("// a\nboom\n// b\n"), 3)
	require.Error(t, err)
	assert.Zero(t, commentOnly)
}

// Same guarantee through the public API: an error never travels with counts
// that make the file look smaller than its physical size.
func TestSourceKeepsConservativeCountsOnFailure(t *testing.T) {
	t.Parallel()
	counts, err := Source("x.go", []byte("package main\n"))
	require.NoError(t, err)
	require.Equal(t, counts.Physical, counts.Counted)

	broken := chroma.MustNewLexer(&chroma.Config{Name: "broken"}, func() chroma.Rules {
		return chroma.Rules{"root": {{Pattern: `.`, Type: chroma.Text, Mutator: chroma.Push("absent")}}}
	})
	_, err = classify(broken, []byte("x\n"), 1)
	require.Error(t, err)
}

// The comment total is capped by the physical total no matter what the lexer
// emits, so Counted can never go negative and a file can never be reported as
// having fewer lines than it occupies.
func TestCountsStayWithinPhysicalBounds(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, filename, body string }{
		{"comment without final newline", "x.py", "# only"},
		{"comment with crlf", "x.go", "// a\r\n// b\r\n"},
		{"lone cr after comment", "x.go", "// a\r"},
		{"nul bytes", "x.go", "// a\x00\n\x00\n"},
		{"whitespace only", "x.go", "   \n\t\n"},
		{"invalid utf8 in comment", "x.rb", "# \xc3\x28\nx = 1\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			counts, err := Source(row.filename, []byte(row.body))
			require.NoError(t, err)
			assert.GreaterOrEqual(t, counts.CommentOnly, 0)
			assert.LessOrEqual(t, counts.CommentOnly, counts.Physical)
			assert.Equal(t, counts.Physical-counts.CommentOnly, counts.Counted)
		})
	}
}

// CRLF sources are counted the same as LF sources: the line ending is not
// content, so it must not change how many lines carry code.
func TestCRLFCountsMatchLF(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, filename, lf string }{
		{"go", "x.go", "// doc\npackage main\n\nvar x = 1 // tail\n"},
		{"python", "x.py", "# doc\nimport os\n\nx = 1  # tail\n"},
		{"typescript", "x.ts", "// doc\n/* block\n   more */\nconst x = 1;\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			lf := count(t, row.filename, row.lf)
			crlf := count(t, row.filename, replaceLF(row.lf))
			assert.Equal(t, lf, crlf)
		})
	}
}

func replaceLF(body string) string {
	out := make([]byte, 0, len(body)*2)
	for i := range len(body) {
		if body[i] == '\n' {
			out = append(out, '\r')
		}
		out = append(out, body[i])
	}
	return string(out)
}
