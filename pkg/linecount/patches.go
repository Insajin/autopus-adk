package linecount

import "github.com/alecthomas/chroma/v2"

// correction prepends rules to one state of a Chroma lexer. Prepending is what
// makes a correction narrow: a rule only competes at offsets where the lexer is
// already in that state, so it cannot reach inside a string or comment the
// grammar handles correctly.
type correction struct {
	state string
	rules []chroma.Rule
}

// correctionsByLanguage repairs the classification failures measured against
// Chroma v2.27.0. The first three are mandatory: Chroma reads a comment marker
// inside a string it does not recognise as a comment, which shrinks the counted
// size of a file that is actually over the limit. The last two recover comment
// syntax Chroma cannot see at all, which inflates it.
//
// Every entry is pinned by a behaviour test, so a Chroma upgrade that fixes one
// upstream keeps passing, and one that renames a state fails loudly.
var correctionsByLanguage = map[string]correction{
	// C++11 raw strings are absent from the C and C++ grammars, so
	// `R"(` ... `)"` is lexed as an identifier followed by an ordinary string
	// and its body as code: every line of `R"(\n// text\n)"` that begins with
	// `//` became a comment. The rule sits in "statements" because the C
	// grammars match string literals there, not in "root", which only
	// dispatches function definitions.
	//
	// C has no raw strings of its own, so the rule can only fire on C++
	// spelled into a .c or .h file, which is the case that needed fixing.
	"C":   {state: "statements", rules: cRawStringRules},
	"C++": {state: "statements", rules: cRawStringRules},

	// C# 11 raw string literals are absent, so `"""` was lexed as an empty
	// string followed by the start of another and the body as code. Three or
	// more quotes in a row cannot be anything else in valid C#: two adjacent
	// string literals need an operator between them. The interpolated form is
	// covered too, because the `$` is a separate token.
	//
	// Verbatim strings (`@"..."`) already lex correctly and are untouched: the
	// grammar consumes them whole, so this rule never sees their interior.
	"C#": {state: "root", rules: []chroma.Rule{{
		Pattern: `("{3,})[\w\W]*?\1`,
		Type:    chroma.LiteralString,
	}}},

	// Ruby heredocs are absent, so the body of `<<~SQL` ... `SQL` was lexed as
	// code and every line starting with `#` became a comment. Heredocs are
	// idiomatic Ruby, which makes this the most likely undercount in practice.
	//
	// The rule is anchored on both ends of the construct: no space is allowed
	// after `<<`, which excludes the far more common append operator
	// (`list << item`), and the terminator has to be a line holding nothing but
	// the tag. A tag that never terminates leaves the source to the unpatched
	// grammar rather than swallowing the rest of the file.
	"Ruby": {state: "root", rules: []chroma.Rule{{
		Pattern: `<<[-~]?(['"]?)([A-Za-z_]\w*)\1[^\n]*\n[\w\W]*?^[ \t]*\2[ \t]*$`,
		Type:    chroma.LiteralStringHeredoc,
	}}},

	// The Sass grammar defines "single-comment" and "multi-comment" states but
	// never pushes either, so an indented-syntax file got no comment credit at
	// all: `//` was lexed as two division operators. Both forms are restored
	// here. Neither can fire inside a string or a url(), because the grammar
	// consumes those whole from the same state.
	"Sass": {state: "root", rules: []chroma.Rule{{
		Pattern: `//[^\n]*`,
		Type:    chroma.CommentSingle,
	}, {
		Pattern: `/\*[\w\W]*?\*/`,
		Type:    chroma.CommentMultiline,
	}}},

	// The Vue grammar emits `<!--` as a comment and then lexes the body as
	// markup, so a multi-line template comment was counted as code. Matching
	// the construct whole fixes it. Script and style blocks are handled in
	// other states, so a `<` in JavaScript is unaffected.
	"vue": {state: "root", rules: []chroma.Rule{{
		Pattern: `<!--[\w\W]*?-->`,
		Type:    chroma.CommentMultiline,
	}}},
}

// cRawStringRules is shared by the C and C++ entries. The delimiter is captured
// and required again at the close, which is what makes the match end at the
// right `)"` instead of the first one.
var cRawStringRules = []chroma.Rule{{
	Pattern: `(?:u8|L|u|U)?R"([^ ()\\\t\n]{0,16})\([\w\W]*?\)\1"`,
	Type:    chroma.LiteralString,
}}
