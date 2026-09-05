package orchestra

import (
	"strings"
	"unicode"
)

// SplitModelSelector separates an OMP provider/model selector from its optional thinking level.
func SplitModelSelector(selector string) (model string, thinking string, ok bool) {
	if selector == "" || strings.Count(selector, "/") != 1 || strings.Count(selector, ":") > 1 {
		return "", "", false
	}
	if strings.IndexFunc(selector, unicode.IsSpace) >= 0 {
		return "", "", false
	}
	provider, modelAndThinking, _ := strings.Cut(selector, "/")
	if strings.Contains(provider, ":") {
		return "", "", false
	}
	if provider == "" || modelAndThinking == "" {
		return "", "", false
	}
	modelID, thinking, hasThinking := strings.Cut(modelAndThinking, ":")
	if modelID == "" || (hasThinking && thinking == "") {
		return "", "", false
	}
	return provider + "/" + modelID, thinking, true
}

// ModelFamilyForSelector returns the canonical model family for a valid selector.
func ModelFamilyForSelector(selector string) string {
	model, _, ok := SplitModelSelector(selector)
	if !ok {
		return ""
	}
	provider, _, _ := strings.Cut(model, "/")
	switch provider {
	case "anthropic":
		return "anthropic"
	case "openai", "openai-codex":
		return "openai"
	case "google", "google-antigravity", "google-vertex":
		return "google"
	default:
		return provider
	}
}
