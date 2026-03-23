// Package config는 autopus.yaml 설정 스키마와 로더를 제공한다.
package config

import "fmt"

// Mode는 설치 모드를 나타낸다.
type Mode string

const (
	ModeFull Mode = "full"
	ModeLite Mode = "lite"
)

// LanguageConf는 프로젝트 언어 설정이다.
type LanguageConf struct {
	Comments    string `yaml:"comments"`     // 코드 주석 언어 (en, ko, ja, zh)
	Commits     string `yaml:"commits"`      // 커밋 메시지 언어
	AIResponses string `yaml:"ai_responses"` // AI 응답 언어
}

// QualityPreset defines a named quality configuration with agent mappings.
type QualityPreset struct {
	Description string            `yaml:"description,omitempty"`
	Agents      map[string]string `yaml:"agents,omitempty"`
}

// QualityConf holds quality preset definitions and the default preset name.
type QualityConf struct {
	Default string                   `yaml:"default,omitempty"`
	Presets map[string]QualityPreset `yaml:"presets,omitempty"`
}

// SkillsConf holds configuration for the skills activation system.
type SkillsConf struct {
	// AutoActivate enables automatic skill activation (default true).
	AutoActivate bool `yaml:"auto_activate"`
	// MaxActiveSkills limits the number of concurrently active skills (default 5).
	MaxActiveSkills int `yaml:"max_active_skills"`
	// CategoryWeights maps category names to priority weights for skill selection.
	CategoryWeights map[string]int `yaml:"category_weights,omitempty"`
}

// TelemetryConf is the pipeline telemetry configuration.
type TelemetryConf struct {
	Enabled       bool `yaml:"enabled"`
	RetentionDays int  `yaml:"retention_days"`
	CostTracking  bool `yaml:"cost_tracking"`
}

// HarnessConfig는 autopus.yaml의 최상위 설정 구조이다.
type HarnessConfig struct {
	Mode          Mode              `yaml:"mode"`
	ProjectName   string            `yaml:"project_name"`
	Platforms     []string          `yaml:"platforms"`
	IsolateRules  bool              `yaml:"isolate_rules,omitempty"`
	Language      LanguageConf      `yaml:"language,omitempty"`
	Architecture  ArchitectureConf  `yaml:"architecture"`
	Lore          LoreConf          `yaml:"lore"`
	Spec          SpecConf          `yaml:"spec"`
	Methodology   MethodologyConf   `yaml:"methodology,omitempty"`
	Router        RouterConf        `yaml:"router,omitempty"`
	Hooks         HooksConf         `yaml:"hooks"`
	Session       SessionConf       `yaml:"session,omitempty"`
	Orchestra     OrchestraConf     `yaml:"orchestra,omitempty"`
	Quality       QualityConf       `yaml:"quality,omitempty"`
	Skills        SkillsConf        `yaml:"skills,omitempty"`
	Verify        VerifyConf        `yaml:"verify,omitempty"`
	Constraints   ConstraintConf    `yaml:"constraints,omitempty"`
	Context       ContextConf       `yaml:"context,omitempty"`
	Telemetry     TelemetryConf     `yaml:"telemetry,omitempty"`
}

// OrchestraConf는 다중 모델 오케스트레이션 설정이다 (Full 전용).
type OrchestraConf struct {
	Enabled         bool                     `yaml:"enabled"`
	DefaultStrategy string                   `yaml:"default_strategy"`
	TimeoutSeconds  int                      `yaml:"timeout_seconds"`
	Providers       map[string]ProviderEntry `yaml:"providers,omitempty"`
	Commands        map[string]CommandEntry  `yaml:"commands,omitempty"`
}

// ProviderEntry는 프로바이더 실행 설정이다.
type ProviderEntry struct {
	Binary        string   `yaml:"binary"`
	Args          []string `yaml:"args,flow"`
	PromptViaArgs bool     `yaml:"prompt_via_args,omitempty"`
}

// CommandEntry는 커맨드별 오케스트레이션 설정이다.
type CommandEntry struct {
	Strategy  string   `yaml:"strategy"`
	Providers []string `yaml:"providers,flow"`
}

// ArchitectureConf는 ARCHITECTURE.md 설정이다.
type ArchitectureConf struct {
	AutoGenerate bool     `yaml:"auto_generate"`
	Enforce      bool     `yaml:"enforce"`
	Layers       []string `yaml:"layers"`
}

// LoreConf는 Lore Decision Knowledge 설정이다.
type LoreConf struct {
	Enabled           bool     `yaml:"enabled"`
	AutoInject        bool     `yaml:"auto_inject"`
	RequiredTrailers  []string `yaml:"required_trailers"`
	StaleThresholdDays int    `yaml:"stale_threshold_days"`
}

// SpecConf는 SPEC 엔진 설정이다.
type SpecConf struct {
	IDFormat   string         `yaml:"id_format"`
	EARSTypes  []string       `yaml:"ears_types"`
	ReviewGate ReviewGateConf `yaml:"review_gate,omitempty"`
}

// ReviewGateConf는 멀티-프로바이더 SPEC 리뷰 게이트 설정이다.
type ReviewGateConf struct {
	Enabled            bool     `yaml:"enabled"`
	Strategy           string   `yaml:"strategy"`
	Providers          []string `yaml:"providers,flow"`
	Judge              string   `yaml:"judge"`
	MaxRevisions       int      `yaml:"max_revisions"`
	AutoCollectContext bool     `yaml:"auto_collect_context"`
	ContextMaxLines    int      `yaml:"context_max_lines"`
}

// MethodologyConf는 방법론 설정이다 (Full 전용).
type MethodologyConf struct {
	Mode       string `yaml:"mode"`
	Enforce    bool   `yaml:"enforce"`
	ReviewGate bool   `yaml:"review_gate"`
}

// RouterConf는 Category-based 모델 라우팅 설정이다 (Full 전용).
type RouterConf struct {
	Strategy   string            `yaml:"strategy"`
	Tiers      map[string]string `yaml:"tiers"`
	Categories map[string]string `yaml:"categories"`
	IntentGate bool              `yaml:"intent_gate"`
}

// HooksConf는 훅 설정이다.
type HooksConf struct {
	PreCommitArch  bool            `yaml:"pre_commit_arch"`
	PreCommitLore  bool            `yaml:"pre_commit_lore"`
	ReactCIFailure bool            `yaml:"react_ci_failure"`
	ReactReview    bool            `yaml:"react_review"`
	Permissions    PermissionsConf `yaml:"permissions,omitempty"`
}

// PermissionsConf는 코딩 CLI 권한 설정이다.
type PermissionsConf struct {
	// ExtraAllow는 autopus.yaml에서 사용자가 추가하는 allow 규칙이다.
	ExtraAllow []string `yaml:"extra_allow,omitempty"`
	// ExtraDeny는 autopus.yaml에서 사용자가 추가하는 deny 규칙이다.
	ExtraDeny []string `yaml:"extra_deny,omitempty"`
}

// SessionConf는 세션 연속성 설정이다 (Full 전용).
type SessionConf struct {
	HandoffEnabled   bool   `yaml:"handoff_enabled"`
	ContinueFile     string `yaml:"continue_file"`
	MaxContextTokens int    `yaml:"max_context_tokens"`
}

// VerifyConf is the frontend UX verification configuration.
type VerifyConf struct {
	Enabled         bool   `yaml:"enabled"`
	DefaultViewport string `yaml:"default_viewport"`
	AutoFix         bool   `yaml:"auto_fix"`
	MaxFixAttempts  int    `yaml:"max_fix_attempts"`
}

// ConstraintConf is the anti-pattern constraint configuration.
type ConstraintConf struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path,omitempty"`
}

// ContextConf is the agent context enrichment configuration.
type ContextConf struct {
	SignatureMap bool `yaml:"signature_map"`
}

// Validate는 설정의 유효성을 검증한다.
func (c *HarnessConfig) Validate() error {
	if c.Mode != ModeFull && c.Mode != ModeLite {
		return fmt.Errorf("invalid mode %q: must be 'full' or 'lite'", c.Mode)
	}
	if c.ProjectName == "" {
		return fmt.Errorf("project_name is required")
	}
	if len(c.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	for _, p := range c.Platforms {
		if !isValidPlatform(p) {
			return fmt.Errorf("invalid platform %q", p)
		}
	}
	if c.Quality.Default != "" {
		if _, ok := c.Quality.Presets[c.Quality.Default]; !ok {
			return fmt.Errorf("quality.default %q is not defined in quality.presets", c.Quality.Default)
		}
	}
	// Validate that each agent model value in quality presets is a known tier.
	validModelTiers := map[string]bool{"opus": true, "sonnet": true, "haiku": true}
	for presetName, preset := range c.Quality.Presets {
		for agentName, tier := range preset.Agents {
			if !validModelTiers[tier] {
				return fmt.Errorf("quality.presets[%s].agents[%s]: unknown model tier %q", presetName, agentName, tier)
			}
		}
	}
	if c.Skills.MaxActiveSkills < 0 {
		return fmt.Errorf("skills.max_active_skills must be non-negative, got %d", c.Skills.MaxActiveSkills)
	}
	return nil
}

// IsFullMode는 Full 모드 여부를 반환한다.
func (c *HarnessConfig) IsFullMode() bool {
	return c.Mode == ModeFull
}

// IsLiteMode는 Lite 모드 여부를 반환한다.
func (c *HarnessConfig) IsLiteMode() bool {
	return c.Mode == ModeLite
}

var validPlatforms = map[string]bool{
	"claude-code": true,
	"codex":       true,
	"gemini-cli":  true,
	"opencode":    true,
	"cursor":      true,
}

func isValidPlatform(p string) bool {
	return validPlatforms[p]
}
