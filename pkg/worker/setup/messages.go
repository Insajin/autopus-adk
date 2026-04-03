package setup

import (
	"fmt"
	"strings"
)

// errorMapping defines a pattern-to-message mapping for user-friendly errors.
type errorMapping struct {
	patterns []string
	message  string
}

// mappings holds all known technical-to-friendly error translations.
var mappings = []errorMapping{
	{
		patterns: []string{"HTTP 500", "HTTP 502", "HTTP 503"},
		message:  "Autopus 서버에 연결할 수 없습니다. 인터넷 연결을 확인하고 다시 시도해주세요.",
	},
	{
		patterns: []string{"PKCE", "code_verifier", "device_code"},
		message:  "서버 인증 중 오류가 발생했습니다. 잠시 후 다시 시도해주세요.",
	},
	{
		patterns: []string{"connection refused", "no such host", "timeout"},
		message:  "네트워크 연결에 실패했습니다. 인터넷 연결을 확인해주세요.",
	},
	{
		patterns: []string{"token expired", "unauthorized", "401"},
		message:  "인증이 만료되었습니다. 다시 로그인해주세요.",
	},
}

// HumanError maps a technical error to a user-friendly message.
// Returns the original error text if no pattern matches.
func HumanError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	for _, m := range mappings {
		for _, p := range m.patterns {
			if strings.Contains(text, p) {
				return m.message
			}
		}
	}
	return text
}

// HumanErrorf wraps HumanError into a new error with the friendly message.
func HumanErrorf(format string, err error) error {
	if err == nil {
		return nil
	}
	friendly := HumanError(err)
	return fmt.Errorf(format, friendly)
}
