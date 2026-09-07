// Package linecount measures source files for the project's per-file size
// limit, reporting how many lines carry code once comment-only lines are set
// aside.
//
// Classification is delegated to Chroma's lexers instead of parsers written
// here: one table maps every extension the limit applies to onto a maintained
// lexer, so a marker inside a string, a template literal, a regex or a heredoc
// is not mistaken for a comment. Chroma has a small set of blind spots that
// would let an oversized file be certified as compliant; each is corrected by
// prepending a narrow rule to the affected lexer state, see patches.go.
//
// Every count is derived from the raw bytes for the physical total and from the
// token stream for the comment total, so a lexer that appends a trailing
// newline or substitutes invalid UTF-8 cannot change the reported size.
//
// Lexing runs at roughly a megabyte per second, so callers enforcing a limit
// should reject or accept on Physical first and only reach for Source when a
// file could still be over the limit once comments come off.
//
// Three residual gaps are known, all of them cases where Chroma reads a
// comment marker that is really content. A JSX, TSX or Vue text child whose
// whole line is "//..." is treated as a comment, though such a line renders as
// visible text and any real content beside it, a URL scheme for instance,
// keeps the line counted. A C string continued across lines with a trailing
// backslash loses its second line the same way. Kotlin nested block comments
// close at the inner delimiter, which counts the outer tail as code: that
// direction is safe, so it is left alone.
package linecount

import (
	"bytes"
	"fmt"
	"os"
)

// Counts is the line accounting for a single source file.
type Counts struct {
	// Physical is the newline-delimited line count. A final line with no
	// trailing newline still counts, and an empty file counts zero.
	Physical int
	// CommentOnly is the number of physical lines whose only non-whitespace
	// content is a language comment, block comment interiors included.
	CommentOnly int
	// Counted is Physical minus CommentOnly: the size the limit applies to.
	Counted int
}

// Source counts data as the language implied by filename's extension.
//
// An extension with no lexer yields Counted == Physical: an unrecognised file
// is measured, not analysed. Any analysis failure is returned as an error
// together with the same conservative counts, so a caller that mishandles the
// error still cannot see a file reported as smaller than it is.
func Source(filename string, data []byte) (Counts, error) {
	physical := Physical(data)
	counts := Counts{Physical: physical, Counted: physical}
	if physical == 0 {
		return counts, nil
	}
	lexer, err := lexerFor(filename)
	if err != nil {
		return counts, err
	}
	if lexer == nil {
		return counts, nil
	}
	commentOnly, err := classify(lexer, data, physical)
	if err != nil {
		return counts, fmt.Errorf("linecount: analyse %q: %w", filename, err)
	}
	counts.CommentOnly = commentOnly
	counts.Counted = physical - commentOnly
	return counts, nil
}

// File reads path and counts it exactly as Source counts in-memory bytes, so a
// file on disk and the same content staged in git produce the same result.
func File(path string) (Counts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Counts{}, fmt.Errorf("linecount: read %q: %w", path, err)
	}
	return Source(path, data)
}

// Physical counts newline-delimited lines without inspecting syntax.
//
// Callers enforcing a line limit should gate on this first: lexing costs
// roughly a megabyte per second, while this is a byte scan, and a file whose
// physical size is already within the limit cannot exceed it once comments are
// excluded.
func Physical(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, newline)
	if data[len(data)-1] != '\n' {
		lines++
	}
	return lines
}

var newline = []byte{'\n'}
