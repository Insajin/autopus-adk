package readiness

import (
	"bytes"
	"encoding/json"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-007: rendered fields keep pre-sanitized HTML entities intact by disabling encoder HTML escaping.
func (r *RenderedValues) MarshalJSON() ([]byte, error) {
	type alias RenderedValues
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if r == nil {
		return []byte("null"), nil
	}
	if err := encoder.Encode(alias(*r)); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}
