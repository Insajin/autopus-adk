package codex

import (
	"bytes"
	"encoding/json"
)

func (d *hooksDoc) UnmarshalJSON(data []byte) error {
	var known struct {
		Description string                `json:"description,omitempty"`
		Hooks       map[string]hookGroups `json:"hooks"`
	}
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	fields, err := decodeJSONObject(data)
	if err != nil {
		return err
	}
	d.Description = known.Description
	d.Hooks = known.Hooks
	d.Extra = jsonExtrasWithout(fields, "description", "hooks")
	return nil
}

func (d hooksDoc) MarshalJSON() ([]byte, error) {
	fields := jsonObjectWithExtras(d.Extra)
	if d.Description != "" {
		fields["description"] = d.Description
	}
	fields["hooks"] = d.Hooks
	return json.Marshal(fields)
}

func (g *hookGroup) UnmarshalJSON(data []byte) error {
	var known struct {
		Matcher             string            `json:"matcher,omitempty"`
		Hooks               json.RawMessage   `json:"hooks"`
		Type                string            `json:"type,omitempty"`
		Command             string            `json:"command"`
		CommandWindows      string            `json:"command_windows,omitempty"`
		CommandWindowsCamel string            `json:"commandWindows,omitempty"`
		Timeout             int               `json:"timeout,omitempty"`
		Env                 map[string]string `json:"env,omitempty"`
		StatusMessage       string            `json:"statusMessage,omitempty"`
		Async               *bool             `json:"async,omitempty"`
		Autopus             bool              `json:"__autopus__,omitempty"`
	}
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	fields, err := decodeJSONObject(data)
	if err != nil {
		return err
	}

	*g = hookGroup{Matcher: known.Matcher, Autopus: known.Autopus}
	if !rawJSONIsNull(known.Hooks) {
		if err := json.Unmarshal(known.Hooks, &g.Hooks); err != nil {
			return err
		}
		g.Extra = jsonExtrasWithout(fields, "matcher", "hooks", "__autopus__")
		return nil
	}

	if known.Command != "" || known.Type != "" {
		g.Hooks = hookHandlers{{
			Type:                known.Type,
			Command:             known.Command,
			CommandWindows:      known.CommandWindows,
			CommandWindowsCamel: known.CommandWindowsCamel,
			Timeout:             known.Timeout,
			Env:                 known.Env,
			StatusMessage:       known.StatusMessage,
			Async:               known.Async,
			Extra: jsonExtrasWithout(fields,
				"matcher", "hooks", "type", "command", "command_windows", "commandWindows",
				"timeout", "env", "statusMessage", "async", "__autopus__"),
			Autopus: known.Autopus,
		}}
		return nil
	}

	g.Extra = jsonExtrasWithout(fields, "matcher", "hooks", "__autopus__")
	return nil
}

func (g hookGroup) MarshalJSON() ([]byte, error) {
	fields := jsonObjectWithExtras(g.Extra)
	if g.Matcher != "" {
		fields["matcher"] = g.Matcher
	}
	fields["hooks"] = g.Hooks
	return json.Marshal(fields)
}

func (h *hookHandler) UnmarshalJSON(data []byte) error {
	var known struct {
		Type                string            `json:"type,omitempty"`
		Command             string            `json:"command"`
		CommandWindows      string            `json:"command_windows,omitempty"`
		CommandWindowsCamel string            `json:"commandWindows,omitempty"`
		Timeout             int               `json:"timeout,omitempty"`
		Env                 map[string]string `json:"env,omitempty"`
		StatusMessage       string            `json:"statusMessage,omitempty"`
		Async               *bool             `json:"async,omitempty"`
		Autopus             bool              `json:"__autopus__,omitempty"`
	}
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	fields, err := decodeJSONObject(data)
	if err != nil {
		return err
	}
	*h = hookHandler{
		Type:                known.Type,
		Command:             known.Command,
		CommandWindows:      known.CommandWindows,
		CommandWindowsCamel: known.CommandWindowsCamel,
		Timeout:             known.Timeout,
		Env:                 known.Env,
		StatusMessage:       known.StatusMessage,
		Async:               known.Async,
		Extra: jsonExtrasWithout(fields,
			"type", "command", "command_windows", "commandWindows", "timeout", "env",
			"statusMessage", "async", "__autopus__"),
		Autopus: known.Autopus,
	}
	return nil
}

func (h hookHandler) MarshalJSON() ([]byte, error) {
	fields := jsonObjectWithExtras(h.Extra)
	if h.Type != "" {
		fields["type"] = h.Type
	}
	fields["command"] = h.Command
	if h.CommandWindows != "" {
		fields["command_windows"] = h.CommandWindows
	}
	if h.CommandWindowsCamel != "" {
		fields["commandWindows"] = h.CommandWindowsCamel
	}
	if h.Timeout != 0 {
		fields["timeout"] = h.Timeout
	}
	if len(h.Env) > 0 {
		fields["env"] = h.Env
	}
	if h.StatusMessage != "" {
		fields["statusMessage"] = h.StatusMessage
	}
	if h.Async != nil {
		fields["async"] = *h.Async
	}
	return json.Marshal(fields)
}

func decodeJSONObject(data []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func jsonExtrasWithout(fields map[string]json.RawMessage, known ...string) map[string]json.RawMessage {
	extras := mergeJSONExtras(nil, fields)
	for _, key := range known {
		delete(extras, key)
	}
	if len(extras) == 0 {
		return nil
	}
	return extras
}

func mergeJSONExtras(base, overlay map[string]json.RawMessage) map[string]json.RawMessage {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	merged := make(map[string]json.RawMessage, len(base)+len(overlay))
	for key, value := range base {
		merged[key] = append(json.RawMessage(nil), value...)
	}
	for key, value := range overlay {
		merged[key] = append(json.RawMessage(nil), value...)
	}
	return merged
}

func jsonObjectWithExtras(extras map[string]json.RawMessage) map[string]any {
	fields := make(map[string]any, len(extras)+2)
	for key, value := range extras {
		fields[key] = value
	}
	return fields
}

func rawJSONIsNull(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}
