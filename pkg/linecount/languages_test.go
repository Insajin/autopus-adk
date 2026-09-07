package linecount

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every extension the size limit applies to has to reach a lexer that actually
// classifies that language. Asserting on behaviour rather than on the name in
// the table is what catches a lexer that resolves but cannot read the syntax:
// each sample carries one comment-only line, one line where a comment shares
// space with code, and one line where a comment marker sits inside a string.
type languageSample struct {
	extension string
	body      string
}

var languageSamples = []languageSample{
	{".c", "/* doc */\n#include <stdio.h>\nint main(void) { puts(\"// keep\"); return 0; } // tail\n"},
	{".h", "/* doc */\n#define GUARD 1\nstatic const char *k = \"// keep\"; // tail\n"},
	{".cc", "// doc\n#include <string>\nstd::string k = \"// keep\"; // tail\n"},
	{".cpp", "// doc\n#include <string>\nstd::string k = \"// keep\"; // tail\n"},
	{".cxx", "// doc\n#include <string>\nstd::string k = \"// keep\"; // tail\n"},
	{".hpp", "// doc\n#pragma once\nconstexpr auto k = \"// keep\"; // tail\n"},
	{".cs", "// doc\nusing System;\nvar k = \"// keep\"; // tail\n"},
	{".css", "/* doc */\na { color: red; }\nb { content: \"/* keep */\"; } /* tail */\n"},
	{".less", "// doc\n@primary: #333;\n.a { content: \"// keep\"; } // tail\n"},
	{".scss", "// doc\n$primary: #333;\n.a { content: \"// keep\"; } // tail\n"},
	{".sass", "// doc\na\n  content: \"// keep\" // tail\n"},
	{".go", "// doc\npackage main\n\nconst k = \"// keep\" // tail\n"},
	{".java", "// doc\nimport java.util.List;\nclass A { String k = \"// keep\"; } // tail\n"},
	{".js", "// doc\nimport fs from \"fs\";\nconst k = \"// keep\"; // tail\n"},
	{".cjs", "// doc\nconst fs = require(\"fs\");\nconst k = \"// keep\"; // tail\n"},
	{".mjs", "// doc\nimport fs from \"fs\";\nconst k = \"// keep\"; // tail\n"},
	{".jsx", "// doc\nimport React from \"react\";\nconst k = \"// keep\"; // tail\n"},
	{".ts", "// doc\nimport fs from \"fs\";\nconst k: string = \"// keep\"; // tail\n"},
	{".tsx", "// doc\nimport React from \"react\";\nconst k: string = \"// keep\"; // tail\n"},
	{".kt", "// doc\nimport kotlin.math.PI\nval k = \"// keep\" // tail\n"},
	{".kts", "// doc\nimport kotlin.math.PI\nval k = \"// keep\" // tail\n"},
	{".php", "<?php\n// doc\nrequire \"a.php\";\n$k = \"// keep\"; // tail\n"},
	{".py", "# doc\nimport os\nk = \"# keep\"  # tail\n"},
	{".rb", "# doc\nrequire \"json\"\nk = \"# keep\" # tail\n"},
	{".rs", "// doc\nuse std::fmt;\nlet k = \"// keep\"; // tail\n"},
	{".sh", "# doc\nset -eu\nk=\"# keep\" # tail\n"},
	{".swift", "// doc\nimport Foundation\nlet k = \"// keep\" // tail\n"},
	{".vue", "<script>\n// doc\nconst k = \"// keep\"; // tail\n</script>\n"},
}

func TestEveryLimitedExtensionClassifiesItsLanguage(t *testing.T) {
	t.Parallel()
	for _, sample := range languageSamples {
		t.Run(sample.extension, func(t *testing.T) {
			t.Parallel()
			language, mapped := Language(sample.extension)
			require.True(t, mapped, "extension is outside the size limit")
			require.NotEmpty(t, language)

			counts := count(t, "sample"+sample.extension, sample.body)
			assert.Equal(t, 1, counts.CommentOnly,
				"exactly the standalone comment line should drop out")
			assert.Equal(t, counts.Physical-1, counts.Counted)
		})
	}
}

// The table is the contract with the arch check; a mapping that points at a
// lexer Chroma does not ship would only surface on a file of that language.
func TestEveryMappedLanguageResolves(t *testing.T) {
	t.Parallel()
	for extension := range languageByExtension {
		t.Run(extension, func(t *testing.T) {
			t.Parallel()
			lexer, err := lexerFor("sample" + extension)
			require.NoError(t, err)
			require.NotNil(t, lexer)
		})
	}
}

func TestExtensionLookupIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	lower, ok := Language(".go")
	require.True(t, ok)
	upper, ok := Language(".GO")
	require.True(t, ok)
	assert.Equal(t, lower, upper)
	assert.Equal(t, 1, count(t, "X.GO", "// doc\npackage main\n").CommentOnly)
}

func TestUnmappedExtensionIsNotClaimed(t *testing.T) {
	t.Parallel()
	_, ok := Language(".md")
	assert.False(t, ok)
	lexer, err := lexerFor("README.md")
	require.NoError(t, err)
	assert.Nil(t, lexer)
}
