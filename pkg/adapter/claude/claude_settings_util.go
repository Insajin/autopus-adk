package claude

// containsHookCommand reports whether any entry in the Stop slice already has
// a nested hook whose "command" field equals the given command string.
// Each entry has the shape: {"matcher": "", "hooks": [{"command": "..."}]}.
// Handles both []any (from JSON unmarshal) and []map[string]any (in-process construction).
func containsHookCommand(entries []any, command string) bool {
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := entry["hooks"].([]any)
		if !ok {
			// Handle []map[string]any produced by in-process construction.
			hooksTyped, ok2 := entry["hooks"].([]map[string]any)
			if !ok2 {
				continue
			}
			for _, h := range hooksTyped {
				if h["command"] == command {
					return true
				}
			}
			continue
		}
		for _, hRaw := range hooks {
			h, ok := hRaw.(map[string]any)
			if !ok {
				continue
			}
			if h["command"] == command {
				return true
			}
		}
	}
	return false
}
