package linecount

import "testing"

// lexicalCase pins how many of a sample's lines must drop out. The expectation
// is a count rather than a token type, so a Chroma upgrade that classifies the
// same source differently still passes as long as the size is right.
type lexicalCase struct {
	name        string
	filename    string
	body        string
	commentOnly int
}

// A comment marker that is really part of a string, a template, a regex, a
// heredoc or rendered text is not a comment. Getting this wrong shrinks a file
// that is over the limit, so these are the cases that matter most.
var markersInsideLiterals = []lexicalCase{{
	name:        "go raw string",
	filename:    "x.go",
	body:        "var s = `\n// not a comment\n/* nor this */\n`\n// real\n",
	commentOnly: 1,
}, {
	name:        "javascript template literal",
	filename:    "x.js",
	body:        "const t = `\n// not a comment\n`;\n// real\n",
	commentOnly: 1,
}, {
	name:        "javascript regex with slashes",
	filename:    "x.js",
	body:        "const re = /https:\\/\\/x/;\nconst re2 = /a\\/\\/b/g;\n// real\n",
	commentOnly: 1,
}, {
	name:        "javascript division is not a comment",
	filename:    "x.js",
	body:        "const q = a / b / c;\n// real\n",
	commentOnly: 1,
}, {
	name:        "python triple quoted string",
	filename:    "x.py",
	body:        "def f():\n    \"\"\"Doc.\n\n    # not a comment\n    \"\"\"\n    return 1  # tail\n# real\n",
	commentOnly: 1,
}, {
	name:        "python docstring is a string not a comment",
	filename:    "x.py",
	body:        "\"\"\"Module doc.\nsecond line.\n\"\"\"\nimport os\n",
	commentOnly: 0,
}, {
	name:        "shell heredoc body",
	filename:    "x.sh",
	body:        "cat <<EOF\n# not a comment\n// nor this\nEOF\n# real\n",
	commentOnly: 1,
}, {
	name:        "shell quoted heredoc body",
	filename:    "x.sh",
	body:        "cat <<'EOF'\n# not a comment\nEOF\n# real\n",
	commentOnly: 1,
}, {
	name:        "ruby squiggly heredoc body",
	filename:    "x.rb",
	body:        "sql = <<~SQL\n  # not a comment\nSQL\n# real\n",
	commentOnly: 1,
}, {
	name:        "ruby plain heredoc body",
	filename:    "x.rb",
	body:        "sql = <<SQL\n# not a comment\nSQL\n# real\n",
	commentOnly: 1,
}, {
	name:        "ruby quoted heredoc body",
	filename:    "x.rb",
	body:        "sql = <<'SQL'\n# not a comment\nSQL\n# real\n",
	commentOnly: 1,
}, {
	name:        "ruby append operator is not a heredoc",
	filename:    "x.rb",
	body:        "list << item\n# real\nitem = 2\n",
	commentOnly: 1,
}, {
	name:        "rust raw string",
	filename:    "x.rs",
	body:        "let s = r#\"\n// not a comment\n\"#;\n// real\n",
	commentOnly: 1,
}, {
	name:        "cpp raw string",
	filename:    "x.cpp",
	body:        "auto s = R\"(\n// not a comment\n)\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "cpp raw string with delimiter",
	filename:    "x.cc",
	body:        "auto s = LR\"tag(\n// not a comment\n)tag\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "c header holding a cpp raw string",
	filename:    "x.h",
	body:        "static const char *s = R\"(\n// not a comment\n)\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "csharp verbatim string",
	filename:    "x.cs",
	body:        "var s = @\"\n// not a comment\n\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "csharp raw string",
	filename:    "x.cs",
	body:        "var s = \"\"\"\n// not a comment\n\"\"\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "csharp interpolated raw string",
	filename:    "x.cs",
	body:        "var s = $\"\"\"\n// not a comment {x}\n\"\"\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "java text block",
	filename:    "x.java",
	body:        "String s = \"\"\"\n// not a comment\n\"\"\";\n// real\n",
	commentOnly: 1,
}, {
	name:        "kotlin raw string",
	filename:    "x.kt",
	body:        "val s = \"\"\"\n// not a comment\n\"\"\"\n// real\n",
	commentOnly: 1,
}, {
	name:        "swift raw string",
	filename:    "x.swift",
	body:        "let s = #\"\"\"\n// not a comment\n\"\"\"#\n// real\n",
	commentOnly: 1,
}, {
	name:        "scss string holding a marker",
	filename:    "x.scss",
	body:        ".a { content: \"// not a comment\"; }\n// real\n",
	commentOnly: 1,
}, {
	name:        "css url holding slashes",
	filename:    "x.css",
	body:        "a { background: url(http://example.test//x); }\n/* real */\n",
	commentOnly: 1,
}}

func TestMarkersInsideLiteralsAreNotComments(t *testing.T) {
	t.Parallel()
	runLexicalCases(t, markersInsideLiterals)
}

// Directives and delimiters that read like comments are executable: dropping
// them would understate a file that genuinely needs those lines.
var executableLookalikes = []lexicalCase{{
	name:        "c preprocessor directives",
	filename:    "x.c",
	body:        "#include <stdio.h>\n#define X 1\n#ifdef X\n#endif\n/* real */\n",
	commentOnly: 1,
}, {
	name:        "cpp pragma",
	filename:    "x.hpp",
	body:        "#pragma once\n#define Y 2\n// real\n",
	commentOnly: 1,
}, {
	name:        "shell shebang",
	filename:    "x.sh",
	body:        "#!/usr/bin/env bash\nset -eu\n# real\n",
	commentOnly: 1,
}, {
	name:        "php tags delimit code",
	filename:    "x.php",
	body:        "<?php\n// real\necho 1;\n?>\n",
	commentOnly: 1,
}}

func TestExecutableLookalikesAreCounted(t *testing.T) {
	t.Parallel()
	runLexicalCases(t, executableLookalikes)
}
