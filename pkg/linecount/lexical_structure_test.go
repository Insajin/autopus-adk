package linecount

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PHP is only PHP between its tags. Text outside them is markup, where a
// comment marker is literal content.
var phpTagBoundary = []lexicalCase{{
	name:        "markers outside php tags are markup",
	filename:    "x.php",
	body:        "// markup text\n<div>\n  # more markup\n</div>\n<?php echo 1; ?>\n",
	commentOnly: 0,
}, {
	name:        "comments inside php tags drop out",
	filename:    "x.php",
	body:        "<?php\n// real\n# also real\n/* and this */\necho 1;\n",
	commentOnly: 3,
}, {
	name:        "markers reappear as markup after the closing tag",
	filename:    "x.php",
	body:        "<?php echo 1; ?>\n// markup again\n",
	commentOnly: 0,
}}

func TestPHPTagBoundary(t *testing.T) {
	t.Parallel()
	runLexicalCases(t, phpTagBoundary)
}

// JSX comments live in an expression container. The braces are code, so the
// line stays counted, and text children are not comments even when they open
// with a URL scheme.
var jsxCases = []lexicalCase{{
	name:        "jsx expression comment shares its line with braces",
	filename:    "x.jsx",
	body:        "const a = (\n  <div>\n    {/* note */}\n  </div>\n);\n",
	commentOnly: 0,
}, {
	name:        "jsx text child holding a url",
	filename:    "x.jsx",
	body:        "const a = (\n  <p>\n    https://example.test/x\n  </p>\n);\n",
	commentOnly: 0,
}, {
	name:        "tsx text child holding a url",
	filename:    "x.tsx",
	body:        "const a = (\n  <p>\n    https://example.test/x\n  </p>\n);\n",
	commentOnly: 0,
}, {
	name:        "tsx comment outside the markup",
	filename:    "x.tsx",
	body:        "// real\nconst a = <p>text</p>;\n",
	commentOnly: 1,
}, {
	name:        "vue template comment",
	filename:    "x.vue",
	body:        "<template>\n  <!-- a comment\n       continued -->\n  <div>x</div>\n</template>\n",
	commentOnly: 2,
}}

func TestJSXAndTemplateComments(t *testing.T) {
	t.Parallel()
	runLexicalCases(t, jsxCases)
}

// Rust and Swift nest block comments; Kotlin's grammar in Chroma does not, and
// the fallback has to be the safe direction. Asserting a bound rather than a
// value keeps this honest without pinning a defect.
func TestNestedBlockComments(t *testing.T) {
	t.Parallel()
	for _, row := range []lexicalCase{
		{name: "rust", filename: "x.rs", body: "fn f() {\n    /* outer /* inner */ still outer */\n}\n", commentOnly: 1},
		{name: "swift", filename: "x.swift", body: "func f() {\n    /* outer /* inner */ still outer */\n}\n", commentOnly: 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, row.commentOnly, count(t, row.filename, row.body).CommentOnly)
		})
	}

	t.Run("kotlin never undercounts", func(t *testing.T) {
		t.Parallel()
		counts := count(t, "x.kt", "fun f() {\n    /* outer /* inner */ still outer */\n}\n")
		assert.Equal(t, 3, counts.Physical)
		assert.LessOrEqual(t, counts.CommentOnly, 1)
	})
}

// Two extensions are read by a grammar that is not their own, because Chroma
// ships no Less lexer and no indented-Sass comment rules. A whole file of
// syntax the borrowed grammar does not share is where that choice would break.
var borrowedGrammars = []lexicalCase{{
	name:     "less mixins guards and escapes",
	filename: "x.less",
	body: "// silent\n@primary: #333;\n@wide: ~\"(min-width: 768px)\";\n" +
		".mixin(@c; @p: 2px) {\n  color: @c;\n}\n" +
		".guard when (@primary > 0) {\n  content: \"// not a comment\";\n}\n" +
		"@media @wide {\n  .a:extend(.b) { .mixin(#fff); } // tail\n}\n" +
		"/* block\n   comment */\n.i { background: url(http://x//y); }\n",
	commentOnly: 3,
}, {
	name:        "less detached ruleset",
	filename:    "x.less",
	body:        "@detached: {\n  // inner\n  background: red;\n};\n.call { @detached(); }\n",
	commentOnly: 1,
}, {
	name:     "sass indented syntax",
	filename: "x.sass",
	body: "// silent\n@import \"base\"\n$primary: #333\n\n.a\n  color: $primary\n" +
		"  &:hover\n    color: darken($primary, 10%) // tail\n\n" +
		"/* css comment\n   continued */\n.b\n  content: \"// not\"\n",
	commentOnly: 3,
}}

func TestBorrowedGrammarsReadTheirFiles(t *testing.T) {
	t.Parallel()
	runLexicalCases(t, borrowedGrammars)
}

// A .php file is markup with PHP embedded in it, so both comment syntaxes have
// to be recognised in their own region and nowhere else.
func TestPHPFileMixesMarkupAndCodeComments(t *testing.T) {
	t.Parallel()
	counts := count(t, "x.php", "<!DOCTYPE html>\n<html>\n  <!-- markup comment -->\n"+
		"    // literal markup text\n  <?php\n    // php comment\n    # php comment\n"+
		"    echo \"// not a comment\";\n  ?>\n</html>\n")
	assert.Equal(t, 10, counts.Physical)
	assert.Equal(t, 3, counts.CommentOnly)
	assert.Equal(t, 7, counts.Counted)
}

func runLexicalCases(t *testing.T, cases []lexicalCase) {
	t.Helper()
	for _, row := range cases {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			counts, err := Source(row.filename, []byte(row.body))
			require.NoError(t, err)
			assert.Equal(t, row.commentOnly, counts.CommentOnly)
			assert.Equal(t, counts.Physical-counts.CommentOnly, counts.Counted)
		})
	}
}
