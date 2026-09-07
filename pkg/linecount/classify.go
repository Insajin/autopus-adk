package linecount

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
)

// classify tokenises data and counts its comment-only lines.
//
// Chroma reports a bad grammar by panicking mid-iteration rather than by
// returning an error, and unmatched input becomes an error token that this
// package reads as code. Neither may take down a caller whose only goal is to
// measure a file, and neither may hand out comment credit from a walk that
// stopped early, so a recovered panic reports zero comments and the failure.
func classify(lexer chroma.Lexer, data []byte, physical int) (commentOnly int, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			commentOnly, err = 0, fmt.Errorf("lexer %q panicked: %v", lexer.Config().Name, recovered)
		}
	}()
	// EnsureLF is left off so CRLF sources keep their line endings; the
	// physical total comes from the raw bytes either way.
	tokens, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, string(data))
	if err != nil {
		return 0, err
	}
	return commentOnlyLines(tokens, physical), nil
}

// commentOnlyLines walks tokens in source order and reports how many of the
// first physical lines hold comment content and no code.
//
// Tokens are emitted in source order, so the current line's state is never
// revisited and no per-line table is allocated regardless of file size. Lines
// beyond physical are discarded: lexers configured with EnsureNL append a
// trailing newline the file does not have.
//
// A line qualifies when it carries no code and either holds comment text or is
// terminated by a newline that belongs to a comment token. The second case is
// what makes a blank line inside a block comment part of the comment, and it
// holds even for lexers that emit a block comment as several tokens.
func commentOnlyLines(tokens chroma.Iterator, physical int) int {
	total, line := 0, 1
	var comment, code bool
	for token := tokens(); token != chroma.EOF; token = tokens() {
		isComment := token.Type.InSubCategory(chroma.Comment)
		segment := token.Value
		for {
			breakAt := strings.IndexByte(segment, '\n')
			text := segment
			if breakAt >= 0 {
				text = segment[:breakAt]
			}
			if strings.TrimSpace(text) != "" {
				if isComment {
					comment = true
				} else {
					code = true
				}
			}
			if breakAt < 0 {
				break
			}
			if !code && (comment || isComment) {
				total++
			}
			line++
			if line > physical {
				return total
			}
			comment, code = false, false
			segment = segment[breakAt+1:]
		}
	}
	// A final line with no trailing newline never reaches the branch above.
	if !code && comment {
		total++
	}
	return total
}
