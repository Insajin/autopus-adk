package cli

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var contextualProviderStatus = regexp.MustCompile(`(?i)\b(?:http|status|error(?:\s+code)?)\s*[:=_-]?\s*([45][0-9]{2})\b`)

var providerFailureTraitOrder = []string{
	"authentication", "authorization_or_entitlement", "model_access", "rate_limit_or_quota",
	"provider_unavailable", "network_transport", "request_validation", "schema_or_response",
}

var providerFailureStatusOrder = []string{"http_4xx", "http_5xx"}

func providerStatusCode(raw json.RawMessage) (int, bool) {
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		value, err := strconv.Atoi(number.String())
		return value, err == nil
	}
	var text string
	if json.Unmarshal(raw, &text) != nil || len(text) != 3 {
		return 0, false
	}
	value, err := strconv.Atoi(text)
	return value, err == nil
}

func addProviderMessageMetadata(state *providerFailureEventState, value string) {
	normalized := strings.ToLower(value)
	for _, trait := range classifyProviderFailureTraits(normalized) {
		state.traits[trait] = true
	}
	if match := contextualProviderStatus.FindStringSubmatch(normalized); len(match) == 2 {
		if code, err := strconv.Atoi(match[1]); err == nil {
			addProviderStatusMetadata(state, code)
		}
	}
}

func addProviderValueTraits(state *providerFailureEventState, value string) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "invalid_token", "authentication_error", "invalid_credentials":
		state.traits["authentication"] = true
	case "permission_denied", "forbidden", "insufficient_permissions":
		state.traits["authorization_or_entitlement"] = true
	case "model_not_found", "unsupported_model", "invalid_model":
		state.traits["model_access"] = true
	case "rate_limit_exceeded", "insufficient_quota":
		state.traits["rate_limit_or_quota"] = true
	case "service_unavailable", "server_error":
		state.traits["provider_unavailable"] = true
	case "invalid_request_error", "unprocessable_entity":
		state.traits["request_validation"] = true
	case "invalid_response", "schema_validation_error":
		state.traits["schema_or_response"] = true
	}
}

func classifyProviderFailureTraits(message string) []string {
	found := make(map[string]bool)
	if containsAny(message, "invalid token", "expired token", "unauthorized", "login required", "not logged in", "invalid credentials", "missing credentials") {
		found["authentication"] = true
	}
	if containsAny(message, "permission denied", "forbidden", "insufficient permission", "not entitled") {
		found["authorization_or_entitlement"] = true
	}
	if strings.Contains(message, "model") && containsAny(message, "does not exist", "do not have access", "don't have access", "not supported", "not allowed", "not available", "unavailable", "not found", "access denied", "unsupported", "invalid model") {
		found["model_access"] = true
	}
	if containsAny(message, "rate limit", "too many requests", "insufficient_quota", "quota exceeded", "usage limit") {
		found["rate_limit_or_quota"] = true
	}
	if containsAny(message, "service unavailable", "temporarily unavailable", "overloaded", "bad gateway", "gateway timeout", "internal server error") {
		found["provider_unavailable"] = true
	}
	if containsAny(message, "stream disconnected", "connection reset", "connection closed", "connection refused", "failed to connect", "unexpected eof", "broken pipe", "network unreachable", "dns", "tls handshake", "timed out", "timeout") {
		found["network_transport"] = true
	}
	if containsAny(message, "bad request", "invalid request", "unprocessable entity", "invalid parameter", "context length", "request too large") {
		found["request_validation"] = true
	}
	if containsAny(message, "output schema", "response format", "parse failure", "parse failed", "no result event", "invalid response", "json schema", "structured output", "response_format", "invalid json") {
		found["schema_or_response"] = true
	}
	return canonicalProviderMetadata(found, providerFailureTraitOrder)
}

func addProviderStatusMetadata(state *providerFailureEventState, code int) {
	switch {
	case code >= 400 && code <= 499:
		state.statusFamilies["http_4xx"] = true
	case code >= 500 && code <= 599:
		state.statusFamilies["http_5xx"] = true
	default:
		return
	}
	switch code {
	case 401:
		state.traits["authentication"] = true
	case 403:
		state.traits["authorization_or_entitlement"] = true
	case 429:
		state.traits["rate_limit_or_quota"] = true
	case 500, 502, 503, 504:
		state.traits["provider_unavailable"] = true
	case 400, 422:
		state.traits["request_validation"] = true
	}
}

func providerOperationalClass(events []providerFailureEventReceipt) string {
	classes := make(map[string]bool)
	for _, event := range events {
		class := providerEventOperationalClass(event.Traits)
		if class == "unknown" {
			return "unknown"
		}
		if class != "" {
			classes[class] = true
		}
	}
	if len(classes) != 1 {
		return "unknown"
	}
	for class := range classes {
		return class
	}
	return "unknown"
}

func providerEventOperationalClass(traits []string) string {
	classes := make(map[string]bool)
	model, entitlement, authentication := false, false, false
	for _, trait := range traits {
		switch trait {
		case "authentication":
			authentication, classes["authentication"] = true, true
		case "authorization_or_entitlement":
			entitlement, classes["authentication"] = true, true
		case "model_access":
			model, classes["model_access"] = true, true
		case "rate_limit_or_quota", "provider_unavailable", "request_validation":
			classes["provider_rejected"] = true
		case "network_transport":
			classes["network_transport"] = true
		case "schema_or_response":
			classes["schema_or_response"] = true
		default:
			return "unknown"
		}
	}
	if model && entitlement && !authentication {
		delete(classes, "authentication")
	}
	if len(classes) == 0 {
		return ""
	}
	if len(classes) != 1 {
		return "unknown"
	}
	for class := range classes {
		return class
	}
	return "unknown"
}
