package codex

import (
	"fmt"
	"strings"
)

type codexTOMLValueState struct {
	quote  byte
	triple bool
	escape bool
	square int
	curly  int
}

// @AX:WARN [AUTO]: structural TOML validation contains more than eight conditional branches.
// @AX:REASON [AUTO]: continuation state, table identity, assignment grammar, duplicate keys, and balanced composite values are admitted fail closed.
func validateCodexTOMLStructure(content string) error {
	var state codexTOMLValueState
	continuation := false
	section := ""
	sections := make(map[string]bool)
	assignments := make(map[string]bool)
	for number, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if continuation {
			state.observe(line)
			if state.square < 0 || state.curly < 0 {
				return fmt.Errorf("line %d: unbalanced value", number+1)
			}
			continuation = state.open()
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if parsed, ok := parseStructuralCodexSection(trimmed); ok {
			if strings.HasPrefix(parsed, "[[") {
				section = fmt.Sprintf("%s#%d", parsed, number)
			} else if sections[parsed] {
				return fmt.Errorf("line %d: duplicate table", number+1)
			} else {
				section, sections[parsed] = parsed, true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			return fmt.Errorf("line %d: invalid table header", number+1)
		}
		equals := strings.IndexByte(trimmed, '=')
		if equals <= 0 || !validCodexTOMLKey(strings.TrimSpace(trimmed[:equals])) {
			return fmt.Errorf("line %d: invalid assignment", number+1)
		}
		value := strings.TrimSpace(trimmed[equals+1:])
		if value == "" {
			return fmt.Errorf("line %d: missing value", number+1)
		}
		assignmentID := section + "\x00" + strings.TrimSpace(trimmed[:equals])
		if assignments[assignmentID] {
			return fmt.Errorf("line %d: duplicate key", number+1)
		}
		assignments[assignmentID] = true
		state = codexTOMLValueState{}
		state.observe(value)
		if state.square < 0 || state.curly < 0 {
			return fmt.Errorf("line %d: unbalanced value", number+1)
		}
		continuation = state.open()
	}
	if continuation {
		return fmt.Errorf("unterminated value")
	}
	return nil
}

func validCodexTOMLKey(key string) bool {
	if key == "" {
		return false
	}
	var quote byte
	for index := range len(key) {
		char := key[index]
		if quote != 0 {
			if char == quote && (quote == '\'' || index == 0 || key[index-1] != '\\') {
				quote = 0
			}
			continue
		}
		switch {
		case char == '"' || char == '\'':
			quote = char
		case char == '.' || char == '-' || char == '_' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z':
		default:
			return false
		}
	}
	return quote == 0
}

func (state *codexTOMLValueState) open() bool {
	return state.quote != 0 || state.square > 0 || state.curly > 0
}

func (state *codexTOMLValueState) observe(value string) {
	for index := 0; index < len(value); index++ {
		char := value[index]
		if state.quote != 0 {
			if state.triple && strings.HasPrefix(value[index:], strings.Repeat(string(state.quote), 3)) {
				state.quote, state.triple = 0, false
				index += 2
				continue
			}
			if !state.triple && char == state.quote && (state.quote == '\'' || !state.escape) {
				state.quote = 0
			}
			state.escape = char == '\\' && !state.escape && state.quote == '"'
			if char != '\\' {
				state.escape = false
			}
			continue
		}
		if char == '#' {
			return
		}
		if char == '"' || char == '\'' {
			state.quote = char
			state.triple = strings.HasPrefix(value[index:], strings.Repeat(string(char), 3))
			if state.triple {
				index += 2
			}
			continue
		}
		switch char {
		case '[':
			state.square++
		case ']':
			state.square--
		case '{':
			state.curly++
		case '}':
			state.curly--
		}
	}
}
