package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (a *Adapter) validateConfig(errs *[]adapter.ValidationError) {
	if !a.managesFile(codexConfigRelPath) {
		return
	}

	data, err := os.ReadFile(filepath.Join(a.root, codexConfigRelPath))
	if err != nil {
		appendConfigReadError(errs, err)
		return
	}

	content := string(data)
	required := []string{"model", "model_reasoning_effort", "approval_policy", "sandbox_mode", "web_search"}
	for _, key := range required {
		if !containsConfigKey(content, key) {
			*errs = append(*errs, adapter.ValidationError{
				File:    codexConfigRelPath,
				Message: fmt.Sprintf("Codex config에 %s 설정이 없음", key),
				Level:   "warning",
			})
		}
	}
	validateDeprecatedConfigKeys(content, errs)
	validateProjectDocBudget(content, errs)
}

func appendConfigReadError(errs *[]adapter.ValidationError, err error) {
	message := ".codex/config.toml을 읽을 수 없음"
	if os.IsNotExist(err) {
		message = ".codex/config.toml이 없음"
	}
	*errs = append(*errs, adapter.ValidationError{
		File:    codexConfigRelPath,
		Message: message,
		Level:   "warning",
	})
}

func validateDeprecatedConfigKeys(content string, errs *[]adapter.ValidationError) {
	if containsConfigKey(content, "approval_mode") {
		*errs = append(*errs, adapter.ValidationError{
			File:    codexConfigRelPath,
			Message: "Codex config가 deprecated approval_mode 키를 사용함: approval_policy로 교체 필요",
			Level:   "warning",
		})
	}
	if strings.Contains(content, "[sandbox]") {
		*errs = append(*errs, adapter.ValidationError{
			File:    codexConfigRelPath,
			Message: "Codex config가 deprecated [sandbox] table을 사용함: sandbox_mode로 교체 필요",
			Level:   "warning",
		})
	}
}

func validateProjectDocBudget(content string, errs *[]adapter.ValidationError) {
	maxBytes, ok := parseProjectDocMaxBytes(content)
	if !ok {
		*errs = append(*errs, adapter.ValidationError{
			File:    codexConfigRelPath,
			Message: "project_doc_max_bytes 설정이 없음",
			Level:   "warning",
		})
		return
	}
	if maxBytes < minProjectDocMaxBytes {
		*errs = append(*errs, adapter.ValidationError{
			File:    codexConfigRelPath,
			Message: fmt.Sprintf("project_doc_max_bytes가 너무 낮음 (%d < %d): 대형 프로젝트 문서가 잘릴 수 있음", maxBytes, minProjectDocMaxBytes),
			Level:   "warning",
		})
	}
}

func containsConfigKey(content, key string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			return true
		}
	}
	return false
}

func parseProjectDocMaxBytes(content string) (int, bool) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "project_doc_max_bytes") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return 0, false
		}
		value, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		return value, err == nil
	}
	return 0, false
}
