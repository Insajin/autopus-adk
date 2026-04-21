// Package config는 autopus.yaml 설정 스키마와 로더를 제공한다.
package config

import "fmt"

// Mode는 설치 모드를 나타낸다.
type Mode string

const (
	ModeFull Mode = "full"
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
	// SharedSurface controls how much of the reusable skill library is published to shared surfaces.
	// full (default): always publish the full shared skill library.
	// auto: full on single-platform installs, core on mixed Codex+OpenCode installs.
	// full: always publish the full shared skill library.
	// core: always publish only the core shared skill set.
	SharedSurface string `yaml:"shared_surface,omitempty"`
	// CategoryWeights maps category names to priority weights for skill selection.
	CategoryWeights map[string]int `yaml:"category_weights,omitempty"`
}

// IssueReportConf is the auto issue reporter configuration.
type IssueReportConf struct {
	Repo             string   `yaml:"repo,omitempty"`
	Labels           []string `yaml:"labels,omitempty"`
	AutoSubmit       bool     `yaml:"auto_submit,omitempty"`
	RateLimitMinutes int      `yaml:"rate_limit_minutes,omitempty"`
}

// HarnessConfig는 autopus.yaml의 최상위 설정 구조이다.
type HarnessConfig struct {
	Mode         Mode             `yaml:"mode"`
	ProjectName  string           `yaml:"project_name"`
	Platforms    []string         `yaml:"platforms"`
	IsolateRules bool             `yaml:"isolate_rules,omitempty"`
	Stack        string           `yaml:"stack,omitempty"`     // detected stack: go, typescript, python, rust
	Framework    string           `yaml:"framework,omitempty"` // detected framework: nextjs, django, gin, etc.
	Language     LanguageConf     `yaml:"language,omitempty"`
	Architecture ArchitectureConf `yaml:"architecture"`
	Lore         LoreConf         `yaml:"lore"`
	Spec         SpecConf         `yaml:"spec"`
	Methodology  MethodologyConf  `yaml:"methodology,omitempty"`
	Router       RouterConf       `yaml:"router,omitempty"`
	Hooks        HooksConf        `yaml:"hooks"`
	Session      SessionConf      `yaml:"session,omitempty"`
	Orchestra    OrchestraConf    `yaml:"orchestra,omitempty"`
	Quality      QualityConf      `yaml:"quality,omitempty"`
	Skills       SkillsConf       `yaml:"skills,omitempty"`
	Verify       VerifyConf       `yaml:"verify,omitempty"`
	Constraints  ConstraintConf   `yaml:"constraints,omitempty"`
	Context      ContextConf      `yaml:"context,omitempty"`
	Features     FeaturesConf     `yaml:"features,omitempty"`
	IssueReport  IssueReportConf  `yaml:"issue_report,omitempty"`
	Profiles     ProfilesConf     `yaml:"profiles,omitempty"`
	UsageProfile UsageProfile     `yaml:"usage_profile,omitempty"` // developer (default) or fullstack
	Hints        HintsConf        `yaml:"hints,omitempty"`
}

// FeaturesConf holds feature-flag namespaces.
type FeaturesConf struct {
	CC21 CC21FeaturesConf `yaml:"cc21,omitempty"`
}

// CC21FeaturesConf holds Claude Code 2.1 integration flags.
type CC21FeaturesConf struct {
	Enabled                 bool   `yaml:"enabled"`
	EffortEnabled           bool   `yaml:"effort_enabled,omitempty"`
	MonitorEnabled          bool   `yaml:"monitor_enabled,omitempty"`
	TaskCreatedEnabled      bool   `yaml:"task_created_enabled,omitempty"`
	InitialPromptEnabled    bool   `yaml:"initial_prompt_enabled,omitempty"`
	TaskCreatedMode         string `yaml:"task_created_mode,omitempty"`
	MonitorPatternTimeoutMS int    `yaml:"monitor_pattern_timeout_ms,omitempty"`
}

// ProfilesConf holds profile configuration for agents.
type ProfilesConf struct {
	Executor ExecutorProfileConf `yaml:"executor,omitempty"`
	Test     TestProfileConf     `yaml:"test,omitempty"`
}

// ExecutorProfileConf holds executor profile settings.
type ExecutorProfileConf struct {
	Default   string                            `yaml:"default,omitempty"`
	CustomDir string                            `yaml:"custom_dir,omitempty"`
	Override  map[string]map[string]interface{} `yaml:"override,omitempty"`
}

// OrchestraConf는 다중 모델 오케스트레이션 설정이다 (Full 전용).
type OrchestraConf struct {
	Enabled            bool                     `yaml:"enabled"`
	DefaultStrategy    string                   `yaml:"default_strategy"`
	TimeoutSeconds     int                      `yaml:"timeout_seconds"`
	Judge              string                   `yaml:"judge,omitempty"`               // global default judge provider for debate
	ConsensusThreshold float64                  `yaml:"consensus_threshold,omitempty"` // global consensus threshold
	Providers          map[string]ProviderEntry `yaml:"providers,omitempty"`
	Commands           map[string]CommandEntry  `yaml:"commands,omitempty"`
	Subprocess         SubprocessConf           `yaml:"subprocess,omitempty"` // global subprocess settings
}

// SubprocessConf holds global subprocess execution settings.
type SubprocessConf struct {
	Enabled       bool   `yaml:"enabled"`                  // enable subprocess mode globally
	MaxConcurrent int    `yaml:"max_concurrent,omitempty"` // max parallel subprocess executions (default 3)
	WorkDir       string `yaml:"work_dir,omitempty"`       // working directory for subprocess execution
	Rounds        int    `yaml:"rounds,omitempty"`         // default debate rounds (default 1)
}

// ProviderEntry는 프로바이더 실행 설정이다.
type ProviderEntry struct {
	Binary           string             `yaml:"binary"`
	Args             []string           `yaml:"args,flow"`
	PaneArgs         []string           `yaml:"pane_args,flow,omitempty"`
	PromptViaArgs    bool               `yaml:"prompt_via_args,omitempty"`
	InteractiveInput string             `yaml:"interactive_input,omitempty"`
	WorkingPatterns  []string           `yaml:"working_patterns,flow,omitempty"`
	Subprocess       SubprocessProvConf `yaml:"subprocess,omitempty"` // per-provider subprocess overrides
}

// SubprocessProvConf holds per-provider subprocess execution overrides.
type SubprocessProvConf struct {
	SchemaFlag   string `yaml:"schema_flag,omitempty"`   // CLI flag name for JSON schema (e.g., "--schema")
	StdinMode    string `yaml:"stdin_mode,omitempty"`    // how to pass prompt: "pipe" (default) or "file"
	OutputFormat string `yaml:"output_format,omitempty"` // expected output format: "json" (default) or "text"
	Timeout      int    `yaml:"timeout,omitempty"`       // per-provider timeout override in seconds
}

// CommandEntry는 커맨드별 오케스트레이션 설정이다.
type CommandEntry struct {
	Strategy           string   `yaml:"strategy"`
	Providers          []string `yaml:"providers,flow"`
	Judge              string   `yaml:"judge,omitempty"`               // provider name to act as debate judge
	ConsensusThreshold float64  `yaml:"consensus_threshold,omitempty"` // per-command consensus threshold override
}

// ArchitectureConf는 ARCHITECTURE.md 설정이다.
type ArchitectureConf struct {
	AutoGenerate bool     `yaml:"auto_generate"`
	Enforce      bool     `yaml:"enforce"`
	Layers       []string `yaml:"layers"`
}

// LoreConf는 Lore Decision Knowledge 설정이다.
type LoreConf struct {
	Enabled            bool     `yaml:"enabled"`
	AutoInject         bool     `yaml:"auto_inject"`
	RequiredTrailers   []string `yaml:"required_trailers"`
	StaleThresholdDays int      `yaml:"stale_threshold_days"`
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
	if c.Mode != ModeFull {
		return fmt.Errorf("invalid mode %q: must be 'full'", c.Mode)
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
	if c.Features.CC21.TaskCreatedMode != "" {
		switch c.Features.CC21.TaskCreatedMode {
		case "warn", "enforce":
		default:
			return fmt.Errorf("features.cc21.task_created_mode %q is invalid", c.Features.CC21.TaskCreatedMode)
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
	switch c.Skills.EffectiveSharedSurface() {
	case SharedSurfaceAuto, SharedSurfaceFull, SharedSurfaceCore:
	default:
		return fmt.Errorf("skills.shared_surface %q is invalid: must be 'auto', 'full', or 'core'", c.Skills.SharedSurface)
	}
	if !c.UsageProfile.IsValid() {
		return fmt.Errorf("invalid usage_profile %q: must be 'developer' or 'fullstack'", c.UsageProfile)
	}
	return nil
}

// IsFullMode는 Full 모드 여부를 반환한다. 항상 true를 반환한다.
func (c *HarnessConfig) IsFullMode() bool {
	return c.Mode == ModeFull
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
