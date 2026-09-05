package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// bareOMPPromptAck reports whether a prompt response carried no payload at
// all, which is how omp 18.1.x acknowledges an accepted prompt.
func bareOMPPromptAck(data json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(data))
	return trimmed == "" || trimmed == "null"
}

func safePipelineOMPToken(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && !strings.ContainsAny(value, "\x00\r\n\t /")
}

func rejectDuplicatePipelineOMPJSON(data []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := scanUniquePipelineOMPJSONValue(decoder); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("OMP pipeline RPC frame contains trailing JSON")
	}
	return nil
}

// @AX:WARN [AUTO]: Recursive JSON structure and duplicate-key validation has cyclomatic complexity 16.
// @AX:REASON [AUTO]: Untrusted RPC frames must reject duplicate authority fields and malformed nested delimiters before decoding.
func scanUniquePipelineOMPJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return errors.New("invalid OMP pipeline RPC JSON structure")
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delimiter == '{' {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || seen[key] {
				return errors.New("OMP pipeline RPC frame contains duplicate or invalid key")
			}
			seen[key] = true
		}
		if err := scanUniquePipelineOMPJSONValue(decoder); err != nil {
			return err
		}
	}
	closing, err := decoder.Token()
	if err != nil || delimiter == '{' && closing != json.Delim('}') || delimiter == '[' && closing != json.Delim(']') {
		return errors.New("invalid OMP pipeline RPC JSON structure")
	}
	return nil
}
