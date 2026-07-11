package codex

import "strings"

type codexTOMLScanState struct {
	multiline string
}

func (s *codexTOMLScanState) skipSyntaxLine(line string) bool {
	if s.multiline == "" {
		return false
	}
	if hasUnescapedToken(line, s.multiline) {
		s.multiline = ""
	}
	return true
}

func (s *codexTOMLScanState) observeValue(value string) {
	value = codexTOMLValueWithoutComment(value)
	for _, delimiter := range []string{`"""`, `'''`} {
		if hasOddUnescapedToken(value, delimiter) {
			s.multiline = delimiter
			return
		}
	}
}

func codexTOMLValueWithoutComment(value string) string {
	const (
		outside = iota
		basic
		literal
		multilineBasic
		multilineLiteral
	)
	state := outside
	escaped := false
	for i := 0; i < len(value); i++ {
		switch state {
		case outside:
			switch {
			case value[i] == '#':
				return value[:i]
			case strings.HasPrefix(value[i:], `"""`):
				state = multilineBasic
				i += 2
			case strings.HasPrefix(value[i:], `'''`):
				state = multilineLiteral
				i += 2
			case value[i] == '"':
				state = basic
			case value[i] == '\'':
				state = literal
			}
		case basic:
			switch {
			case escaped:
				escaped = false
			case value[i] == '\\':
				escaped = true
			case value[i] == '"':
				state = outside
			}
		case literal:
			if value[i] == '\'' {
				state = outside
			}
		case multilineBasic:
			if strings.HasPrefix(value[i:], `"""`) && !isEscapedAt(value, i) {
				state = outside
				i += 2
			}
		case multilineLiteral:
			if strings.HasPrefix(value[i:], `'''`) {
				state = outside
				i += 2
			}
		}
	}
	return value
}

func hasUnescapedToken(value, token string) bool {
	for offset := 0; offset < len(value); {
		index := strings.Index(value[offset:], token)
		if index < 0 {
			return false
		}
		index += offset
		if !isEscapedAt(value, index) {
			return true
		}
		offset = index + len(token)
	}
	return false
}

func isEscapedAt(value string, index int) bool {
	backslashes := 0
	for i := index - 1; i >= 0 && value[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func hasOddUnescapedToken(value, token string) bool {
	count := 0
	for offset := 0; offset < len(value); {
		index := strings.Index(value[offset:], token)
		if index < 0 {
			break
		}
		index += offset
		if !isEscapedAt(value, index) {
			count++
		}
		offset = index + len(token)
	}
	return count%2 == 1
}
