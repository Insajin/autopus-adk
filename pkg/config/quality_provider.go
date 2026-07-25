package config

import "strings"

const (
	QualityProviderClaude = "claude"
	QualityProviderCodex  = "codex"
	maxQualityPresetName  = 64
)

// NormalizeQualityProvider returns the canonical provider key accepted by
// quality.providers. Claude Code is accepted as a CLI-facing alias.
// @AX:ANCHOR [AUTO]: Preserve this canonical provider-key normalization contract. @AX:SPEC SPEC-PROVIDER-QUALITY-001
// @AX:REASON [AUTO]: Four production callers share the same accepted aliases across validation, effective resolution, and CLI persistence.
func NormalizeQualityProvider(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case QualityProviderClaude, "claude-code":
		return QualityProviderClaude, true
	case QualityProviderCodex:
		return QualityProviderCodex, true
	default:
		return "", false
	}
}

func normalizeQualityProviderKey(raw string) string {
	provider, ok := NormalizeQualityProvider(raw)
	if !ok {
		return ""
	}
	return provider
}

// IsValidQualityPresetName reports whether name is safe to persist as a
// command argument and YAML scalar identifier.
func IsValidQualityPresetName(name string) bool {
	if len(name) == 0 || len(name) > maxQualityPresetName {
		return false
	}
	for i := 0; i < len(name); i++ {
		char := name[i]
		alphanumeric := char >= 'a' && char <= 'z' ||
			char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9'
		if !alphanumeric && (i == 0 || char != '-' && char != '_') {
			return false
		}
	}
	return true
}

// EffectiveMode resolves a provider override, then the global default, then
// the backward-compatible balanced safety fallback.
// @AX:ANCHOR [AUTO]: Preserve provider override, global default, and balanced fallback precedence. @AX:SPEC SPEC-PROVIDER-QUALITY-001
// @AX:REASON [AUTO]: Three production callers rely on this resolution order for provider views and CLI status after persistence.
func (q QualityConf) EffectiveMode(provider string) string {
	if canonical, ok := NormalizeQualityProvider(provider); ok {
		if mode := strings.TrimSpace(q.Providers[canonical]); q.isKnownQualityMode(mode) {
			return mode
		}
	}
	if mode := strings.TrimSpace(q.Default); q.isKnownQualityMode(mode) {
		return mode
	}
	return "balanced"
}

func (q QualityConf) isKnownQualityMode(mode string) bool {
	if mode == "ultra" || mode == "balanced" {
		return true
	}
	_, ok := q.Presets[mode]
	return mode != "" && ok
}

// ForProvider returns a detached view whose Default is the effective provider
// mode. Provider overrides are cleared so downstream legacy consumers cannot
// accidentally resolve a different provider a second time.
func (q QualityConf) ForProvider(provider string) QualityConf {
	effective := q
	effective.Default = q.EffectiveMode(provider)
	effective.Providers = nil
	return effective
}

// WithGlobalOverride returns a detached per-run view in which an explicit
// global --quality value wins over every persisted provider override.
func (q QualityConf) WithGlobalOverride(mode string) QualityConf {
	effective := q
	effective.Default = strings.TrimSpace(mode)
	effective.Providers = nil
	return effective
}
