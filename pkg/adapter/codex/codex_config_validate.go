package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
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
	if err := validateCodexTOMLStructure(content); err != nil {
		*errs = append(*errs, adapter.ValidationError{
			File: codexConfigRelPath, Message: "Codex config TOML이 malformed 상태임", Level: "error",
		})
		return
	}
	required := []string{"approval_policy", "sandbox_mode", "web_search"}
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
	validateCodexFeatureFlags(content, errs)
	validateBundledCodexPlugins(content, errs)
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
	for section, keys := range codexObsoleteConfigKeys {
		for key := range keys {
			if !sectionHasConfigKey(content, section, key) {
				continue
			}
			*errs = append(*errs, adapter.ValidationError{
				File:    codexConfigRelPath,
				Message: fmt.Sprintf("Codex config가 obsolete %s.%s 키를 사용함", section, key),
				Level:   "warning",
			})
		}
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

func validateBundledCodexPlugins(content string, errs *[]adapter.ValidationError) {
	if sectionHasEnabledTrue(content, `plugins."browser@openai-bundled"`) {
		return
	}
	*errs = append(*errs, adapter.ValidationError{
		File:    codexConfigRelPath,
		Message: "Codex bundled browser plugin이 enabled 상태가 아님",
		Level:   "warning",
	})
}

func validateCodexFeatureFlags(content string, errs *[]adapter.ValidationError) {
	required := map[string]string{
		"goals":        "Codex goals feature가 enabled 상태가 아님",
		"hooks":        "Codex hooks feature가 enabled 상태가 아님",
		"shell_tool":   "Codex shell_tool feature가 enabled 상태가 아님",
		"unified_exec": "Codex unified_exec feature가 enabled 상태가 아님",
	}
	for key, message := range required {
		if sectionHasKeyValue(content, "features", key, "true") {
			continue
		}
		*errs = append(*errs, adapter.ValidationError{
			File: codexConfigRelPath, Message: message, Level: "warning",
		})
	}
	if !sectionHasKeyValue(content, "features.multi_agent_v2", "enabled", "true") {
		*errs = append(*errs, adapter.ValidationError{
			File:    codexConfigRelPath,
			Message: "Codex multi_agent_v2 feature가 enabled 상태가 아님",
			Level:   "error",
		})
	}
	validateCodexAgentConcurrency(content, errs)
}

// validateCodexAgentConcurrency accepts the ceiling under any name a config
// file can carry it: the documented key, its documented legacy alias, and the
// undocumented table an older Autopus wrote. Only the documented key is ever
// generated, but validation reads the file on disk, and a project that has not
// been regenerated yet still configures a real ceiling. The value is checked
// against the harness bounds rather than against a fixed 4, which is now a
// default the project may override.
func validateCodexAgentConcurrency(content string, errs *[]adapter.ValidationError) {
	for _, source := range agentConcurrencySources() {
		raw, ok := sectionConfigValue(content, source.namespace, source.key)
		if !ok {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < config.CodexAgentConcurrencyMin || value > config.CodexAgentConcurrencyMax {
			*errs = append(*errs, adapter.ValidationError{
				File: codexConfigRelPath,
				Message: fmt.Sprintf("Codex agent concurrency 값이 유효 범위(%d-%d)를 벗어남: %s",
					config.CodexAgentConcurrencyMin, config.CodexAgentConcurrencyMax, raw),
				Level: "error",
			})
		}
		return
	}
	*errs = append(*errs, adapter.ValidationError{
		File:    codexConfigRelPath,
		Message: "Codex agent concurrency 설정이 없음: 'auto update' 실행 필요",
		Level:   "error",
	})
}

func sectionHasEnabledTrue(content, wantSection string) bool {
	return sectionHasKeyValue(content, wantSection, "enabled", "true")
}

func sectionHasKeyValue(content, wantSection, wantKey, wantValue string) bool {
	var section string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if parsedSection, ok := parseCodexConfigSection(trimmed); ok {
			section = parsedSection
			continue
		}
		if section != wantSection {
			continue
		}
		key, value, ok := parseCodexConfigAssignment(trimmed)
		if ok && key == wantKey && value == wantValue {
			return true
		}
	}
	return false
}

func sectionHasConfigKey(content, wantSection, wantKey string) bool {
	_, ok := sectionConfigValue(content, wantSection, wantKey)
	return ok
}

// sectionConfigValue returns the assigned value of one key inside one section
// with any trailing comment removed, so a caller can parse the value itself.
func sectionConfigValue(content, wantSection, wantKey string) (string, bool) {
	var section string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if parsed, ok := parseCodexConfigSection(trimmed); ok {
			section = parsed
			continue
		}
		key, value, ok := parseCodexConfigAssignment(trimmed)
		if section == wantSection && ok && key == wantKey {
			return strings.TrimSpace(codexTOMLValueWithoutComment(value)), true
		}
	}
	return "", false
}
