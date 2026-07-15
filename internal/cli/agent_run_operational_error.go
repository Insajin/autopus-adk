package cli

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const maxOperationalErrorBytes = 64 * 1024

type boundedOperationalErrorBuffer struct {
	data []byte
}

func (b *boundedOperationalErrorBuffer) Write(input []byte) (int, error) {
	written := len(input)
	remaining := maxOperationalErrorBytes - len(b.data)
	if remaining > 0 {
		if len(input) > remaining {
			input = input[:remaining]
		}
		b.data = append(b.data, input...)
	}
	return written, nil
}

func (b *boundedOperationalErrorBuffer) String() string { return string(b.data) }

func (b *boundedOperationalErrorBuffer) HasData() bool { return len(b.data) > 0 }

func operationalErrorSignals(stderrObserved, providerFailureEvent, streamParseFailed bool) []string {
	var signals []string
	if stderrObserved {
		signals = append(signals, "stderr")
	}
	if providerFailureEvent {
		signals = append(signals, "provider_failure_event")
	}
	if streamParseFailed {
		signals = append(signals, "stream_parse_failure")
	}
	return signals
}

func classifyOperationalError(stderr string, cause error) (string, string) {
	message := strings.ToLower(stderr)
	if cause != nil {
		message += "\n" + strings.ToLower(cause.Error())
	}
	return fingerprintOperationalErrorClass(operationalErrorClass(message))
}

func classifyOperationalErrorWithProvider(stderr string, cause error, events []providerFailureEventReceipt) (string, string) {
	localClass, _ := classifyOperationalError(stderr, cause)
	providerClass := providerOperationalClass(events)
	class := mergeOperationalErrorClasses(localClass, providerClass, len(events) > 0)
	return fingerprintOperationalErrorClass(class)
}

func mergeOperationalErrorClasses(localClass, providerClass string, providerObserved bool) string {
	if !providerObserved {
		if localClass != "" {
			return localClass
		}
		return "unknown"
	}
	if providerClass == "" || providerClass == "unknown" {
		return "unknown"
	}
	if localClass == "" || localClass == "unknown" {
		return providerClass
	}
	if localClass == providerClass {
		return localClass
	}
	return "unknown"
}

func fingerprintOperationalErrorClass(class string) (string, string) {
	digest := sha256.Sum256([]byte("autopus-operational-error-v1\x00" + class))
	return class, fmt.Sprintf("sha256:%x", digest)
}

func operationalErrorClass(message string) string {
	switch {
	case containsAny(message, "executable file not found", "command not found") ||
		(strings.Contains(message, "fork/exec") && strings.Contains(message, "no such file or directory")):
		return "binary_missing"
	case containsAny(message, "model") && containsAny(message, "not available", "not found", "unsupported", "access denied"):
		return "model_access"
	case containsAny(message, "unauthorized", "authentication", "login required", "not logged in", "api key") ||
		containsNumericCode(message, "401"):
		return "authentication"
	case containsAny(message, "unknown argument", "unexpected argument", "invalid value", "invalid config", "configuration error", "toml parse"):
		return "cli_usage_or_config"
	case containsAny(message, "connection refused", "network is unreachable", "dns", "tls", "handshake", "timed out", "timeout", "proxy error"):
		return "network_transport"
	case containsAny(message, "rate limit", "quota", "too many requests", "provider rejected", "service unavailable") ||
		containsNumericCode(message, "429") || containsNumericCode(message, "503"):
		return "provider_rejected"
	case containsAny(message, "output schema", "response format", "parse failure", "parse failed", "no result event", "invalid response"):
		return "schema_or_response"
	default:
		return "unknown"
	}
}

func containsNumericCode(value, code string) bool {
	for start := 0; ; {
		index := strings.Index(value[start:], code)
		if index < 0 {
			return false
		}
		index += start
		beforeOK := index == 0 || value[index-1] < '0' || value[index-1] > '9'
		after := index + len(code)
		afterOK := after == len(value) || value[after] < '0' || value[after] > '9'
		if beforeOK && afterOK {
			return true
		}
		start = index + 1
	}
}

func containsAny(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}
