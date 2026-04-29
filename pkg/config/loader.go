package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const configFileName = "autopus.yaml"

// Load는 autopus.yaml을 로드한다. 파일이 없으면 기본 설정을 반환한다.
func Load(dir string) (*HarnessConfig, error) {
	cfg, _, err := loadConfig(dir, true)
	return cfg, err
}

// LoadPreview는 autopus.yaml을 로드하되, 파일 정규화 결과를 디스크에 쓰지 않는다.
func LoadPreview(dir string) (*HarnessConfig, error) {
	cfg, _, err := loadConfig(dir, false)
	return cfg, err
}

// LoadPreviewWithMetadata는 preview 로드와 함께 정규화 필요 여부를 반환한다.
func LoadPreviewWithMetadata(dir string) (*HarnessConfig, bool, error) {
	return loadConfig(dir, false)
}

func loadConfig(dir string, persistNormalization bool) (*HarnessConfig, bool, error) {
	path := filepath.Join(dir, configFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			name := filepath.Base(dir)
			return DefaultFullConfig(name), false, nil
		}
		return nil, false, fmt.Errorf("read config: %w", err)
	}

	expanded := expandEnvVars(string(data))

	var cfg HarnessConfig
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, false, fmt.Errorf("parse config: %w", err)
	}
	applyMissingDefaults(&cfg, []byte(expanded))

	normalized := MigratePlatformNames(&cfg)
	if persistNormalization && normalized {
		// Persist the corrected config so subsequent loads don't repeat the migration.
		if corrected, marshalErr := yaml.Marshal(&cfg); marshalErr == nil {
			_ = os.WriteFile(path, corrected, 0644)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, false, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, normalized, nil
}

// @AX:NOTE [AUTO]: Missing design block is backfilled for backward compatibility; explicit design config keeps caller intent.
func applyMissingDefaults(cfg *HarnessConfig, data []byte) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	defaults := DefaultFullConfig(cfg.ProjectName)
	if _, ok := raw["design"]; !ok {
		cfg.Design = defaults.Design
	}
}

// Save validates and writes the config to autopus.yaml.
func Save(dir string, cfg *HarnessConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := filepath.Join(dir, configFileName)
	return os.WriteFile(path, data, 0644)
}

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return match
	})
}
