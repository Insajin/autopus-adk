package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// PlatformSettings is the canonical cross-platform settings representation.
// Each platform adapter can serialize from this struct into its native format.
type PlatformSettings struct {
	Agents []AgentDef `json:"agents"`
	Rules  []RuleDef  `json:"rules"`
	Skills []SkillDef `json:"skills"`
	Hooks  []HookDef  `json:"hooks"`
}

// AgentDef is a platform-agnostic agent definition.
type AgentDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
}

// RuleDef is a platform-agnostic rule definition.
type RuleDef struct {
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Content  string `json:"content"`
}

// SkillDef is a platform-agnostic skill definition.
type SkillDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category,omitempty"`
}

// HookDef is a platform-agnostic hook definition.
type HookDef struct {
	Event   string `json:"event"`
	Matcher string `json:"matcher"`
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// ToClaudeJSON serializes to Claude's nested JSON format (.claude/settings.json style).
func (ps *PlatformSettings) ToClaudeJSON() ([]byte, error) {
	out := make(map[string]any)

	// Hooks: nested map keyed by event
	if len(ps.Hooks) > 0 {
		hooksMap := make(map[string]any)
		for _, h := range ps.Hooks {
			entry := map[string]any{
				"matcher": h.Matcher,
				"hooks": []map[string]any{{
					"type":    "command",
					"command": h.Command,
				}},
			}
			if h.Timeout > 0 {
				entry["hooks"].([]map[string]any)[0]["timeout"] = h.Timeout
			}
			hooksMap[h.Event] = entry
		}
		out["hooks"] = hooksMap
	}

	// Agents: array under "agents" key
	if len(ps.Agents) > 0 {
		out["agents"] = ps.Agents
	}

	// Rules and Skills stored as metadata references
	if len(ps.Rules) > 0 {
		out["rules"] = ps.Rules
	}
	if len(ps.Skills) > 0 {
		out["skills"] = ps.Skills
	}

	return marshalIndent(out)
}

// ToGeminiJSON serializes to Gemini's flat JSON format (.gemini/settings.json style).
func (ps *PlatformSettings) ToGeminiJSON() ([]byte, error) {
	out := make(map[string]any)

	// Gemini uses flat arrays for all categories
	if len(ps.Agents) > 0 {
		out["agents"] = ps.Agents
	}
	if len(ps.Rules) > 0 {
		names := make([]string, len(ps.Rules))
		for i, r := range ps.Rules {
			names[i] = r.Name
		}
		out["rules"] = names
	}
	if len(ps.Skills) > 0 {
		names := make([]string, len(ps.Skills))
		for i, s := range ps.Skills {
			names[i] = s.Name
		}
		out["skills"] = names
	}
	if len(ps.Hooks) > 0 {
		out["hooks"] = ps.Hooks
	}

	return marshalIndent(out)
}

// ToCodexConfig serializes the agent portion to Codex TOML format.
// Codex uses TOML for agents, JSON for hooks, and MD for AGENTS.md.
// This method produces the TOML agent config portion.
func (ps *PlatformSettings) ToCodexConfig() ([]byte, error) {
	var buf bytes.Buffer

	for i, agent := range ps.Agents {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("name = %q\n", agent.Name))
		buf.WriteString(fmt.Sprintf("description = %q\n", agent.Description))
		if agent.Model != "" {
			buf.WriteString(fmt.Sprintf("model = %q\n", agent.Model))
		}
	}

	return buf.Bytes(), nil
}

// ParseClaudeJSON deserializes Claude's nested JSON format back to PlatformSettings.
func ParseClaudeJSON(data []byte) (*PlatformSettings, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("claude JSON parse failed: %w", err)
	}

	ps := &PlatformSettings{}

	if agentsRaw, ok := raw["agents"]; ok {
		if err := json.Unmarshal(agentsRaw, &ps.Agents); err != nil {
			return nil, fmt.Errorf("agents parse failed: %w", err)
		}
	}
	if rulesRaw, ok := raw["rules"]; ok {
		if err := json.Unmarshal(rulesRaw, &ps.Rules); err != nil {
			return nil, fmt.Errorf("rules parse failed: %w", err)
		}
	}
	if skillsRaw, ok := raw["skills"]; ok {
		if err := json.Unmarshal(skillsRaw, &ps.Skills); err != nil {
			return nil, fmt.Errorf("skills parse failed: %w", err)
		}
	}
	if hooksRaw, ok := raw["hooks"]; ok {
		var hooksMap map[string]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
				Timeout int    `json:"timeout,omitempty"`
			} `json:"hooks"`
		}
		if err := json.Unmarshal(hooksRaw, &hooksMap); err != nil {
			return nil, fmt.Errorf("hooks parse failed: %w", err)
		}
		for event, entry := range hooksMap {
			for _, h := range entry.Hooks {
				ps.Hooks = append(ps.Hooks, HookDef{
					Event:   event,
					Matcher: entry.Matcher,
					Command: h.Command,
					Timeout: h.Timeout,
				})
			}
		}
	}

	return ps, nil
}

// ParseGeminiJSON deserializes Gemini's flat JSON format back to PlatformSettings.
func ParseGeminiJSON(data []byte) (*PlatformSettings, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("gemini JSON parse failed: %w", err)
	}

	ps := &PlatformSettings{}

	if agentsRaw, ok := raw["agents"]; ok {
		if err := json.Unmarshal(agentsRaw, &ps.Agents); err != nil {
			return nil, fmt.Errorf("agents parse failed: %w", err)
		}
	}
	if rulesRaw, ok := raw["rules"]; ok {
		var names []string
		if err := json.Unmarshal(rulesRaw, &names); err != nil {
			return nil, fmt.Errorf("rules parse failed: %w", err)
		}
		for _, n := range names {
			ps.Rules = append(ps.Rules, RuleDef{Name: n})
		}
	}
	if skillsRaw, ok := raw["skills"]; ok {
		var names []string
		if err := json.Unmarshal(skillsRaw, &names); err != nil {
			return nil, fmt.Errorf("skills parse failed: %w", err)
		}
		for _, n := range names {
			ps.Skills = append(ps.Skills, SkillDef{Name: n})
		}
	}
	if hooksRaw, ok := raw["hooks"]; ok {
		if err := json.Unmarshal(hooksRaw, &ps.Hooks); err != nil {
			return nil, fmt.Errorf("hooks parse failed: %w", err)
		}
	}

	return ps, nil
}

// ParseCodexConfig deserializes Codex TOML agent config back to PlatformSettings.
// Only agents are parsed (Codex TOML only contains agent definitions).
func ParseCodexConfig(data []byte) (*PlatformSettings, error) {
	ps := &PlatformSettings{}
	agent := AgentDef{}
	hasAgent := false

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if hasAgent {
				ps.Agents = append(ps.Agents, agent)
				agent = AgentDef{}
				hasAgent = false
			}
			continue
		}

		key, val, ok := parseTOMLLine(line)
		if !ok {
			continue
		}
		hasAgent = true
		switch key {
		case "name":
			agent.Name = val
		case "description":
			agent.Description = val
		case "model":
			agent.Model = val
		}
	}
	if hasAgent {
		ps.Agents = append(ps.Agents, agent)
	}

	return ps, nil
}

// parseTOMLLine extracts key and unquoted value from a simple TOML line.
func parseTOMLLine(line string) (key, val string, ok bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key = strings.TrimSpace(parts[0])
	val = strings.TrimSpace(parts[1])
	// Unquote if quoted
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	return key, val, true
}

func marshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
