package config

const (
	CodexFrontierModel           = "gpt-5.5"
	CodexStandardModel           = CodexFrontierModel
	CodexMiniModel               = CodexFrontierModel
	CodexCodingModel             = CodexFrontierModel
	CodexSparkModel              = CodexFrontierModel
	CodexFallbackModel           = CodexFrontierModel
	CodexOrchestraTimeoutSeconds = 420
	// ClaudeOrchestraTimeoutSeconds covers opus reasoning that routinely runs
	// 3–6 minutes on spec review workloads. Exceeds the 240s global timeout to
	// prevent the cutoff reported in issue #55.
	ClaudeOrchestraTimeoutSeconds = 480
)

// DefaultCodexProviderEntry returns the canonical Codex orchestra provider entry.
func DefaultCodexProviderEntry() ProviderEntry {
	return ProviderEntry{
		Binary: "codex",
		// SPEC-ORCH-021 REQ-014/015: `exec --sandbox workspace-write` (no deprecated --full-auto);
		// reasoning effort aligned to autopus.yaml. Pane argv stays interactive (no leading `exec`).
		Args:          []string{"exec", "--sandbox", "workspace-write", "-m", CodexFrontierModel, "-c", `model_reasoning_effort="xhigh"`},
		PaneArgs:      []string{"-m", CodexFrontierModel, "-c", `model_reasoning_effort="xhigh"`},
		PromptViaArgs: false,
		Subprocess: SubprocessProvConf{
			SchemaFlag: "--output-schema",
			Timeout:    CodexOrchestraTimeoutSeconds,
		},
	}
}

// DefaultFullConfig returns the default config for Full mode.
// @AX:NOTE: [AUTO] magic constants — model names (opus, sonnet, haiku, Codex GPT-5.x), timeouts, and tier mappings are hardcoded below
func DefaultFullConfig(projectName string) *HarnessConfig {
	return &HarnessConfig{
		Mode:        ModeFull,
		ProjectName: projectName,
		Platforms:   []string{"claude-code"},
		Architecture: ArchitectureConf{
			AutoGenerate: true,
			Enforce:      true,
		},
		Lore: LoreConf{
			Enabled:            true,
			RequiredTrailers:   []string{"Constraint"},
			StaleThresholdDays: 90,
		},
		Spec: SpecConf{
			IDFormat:  "SPEC-{DOMAIN}-{NUMBER}",
			EARSTypes: []string{"ubiquitous", "event-driven", "unwanted", "optional", "complex"},
			ReviewGate: ReviewGateConf{
				Enabled:  true,
				Strategy: "debate",
				// SPEC-ORCH-021 REQ-016: include codex so it is not silently dropped from
				// structured review; matches the orchestra command provider set.
				Providers:          []string{"claude", "codex", "gemini"},
				Judge:              "claude",
				MaxRevisions:       2,
				AutoCollectContext: true,
				ContextMaxLines:    0,
				VerdictThreshold:   0.67,
				DocContextMaxLines: 200,
			},
		},
		Methodology: MethodologyConf{
			Mode:       "tdd",
			Enforce:    true,
			ReviewGate: true,
		},
		Router: RouterConf{
			Strategy: "balanced",
			Tiers: map[string]string{
				"premium":  "claude-opus-4-8",
				"standard": "claude-sonnet-4-6",
				"economy":  "claude-sonnet-4-6",
			},
			Categories: map[string]string{
				"visual":     "standard",
				"deep":       "premium",
				"quick":      "economy",
				"ultrabrain": "premium",
				"writing":    "standard",
				"git":        "economy",
			},
			IntentGate: true,
		},
		Hooks: HooksConf{
			PreCommitArch:  true,
			PreCommitLore:  true,
			ReactCIFailure: true,
			ReactReview:    true,
		},
		Session: SessionConf{
			HandoffEnabled:   true,
			ContinueFile:     ".auto-continue.md",
			MaxContextTokens: 2000,
		},
		Orchestra: OrchestraConf{
			Enabled:         true,
			DefaultStrategy: "consensus",
			TimeoutSeconds:  240,
			Judge:           "claude",
			Providers: map[string]ProviderEntry{
				"claude": {
					Binary:     "claude",
					Args:       []string{"--print", "--model", "opus", "--effort", "high"},
					PaneArgs:   []string{"--print", "--model", "opus", "--effort", "high"},
					Subprocess: SubprocessProvConf{Timeout: ClaudeOrchestraTimeoutSeconds},
				},
				// SPEC-ORCH-021 REQ-014/015: prompt is the value of --print (injected into the ""
				// slot); pane argv carries no --print (interactive session).
				"gemini": {Binary: "agy", Args: []string{"--print", ""}, PaneArgs: []string{}, PromptViaArgs: true, InteractiveInput: "stdin", Subprocess: SubprocessProvConf{OutputFormat: "text"}},
				"codex":  DefaultCodexProviderEntry(),
			},
			Commands: map[string]CommandEntry{
				"review":     {Strategy: "debate", Providers: []string{"claude", "codex", "gemini"}},
				"plan":       {Strategy: "consensus", Providers: []string{"claude", "codex", "gemini"}},
				"secure":     {Strategy: "consensus", Providers: []string{"claude", "codex", "gemini"}},
				"brainstorm": {Strategy: "debate", Providers: []string{"claude", "codex", "gemini"}},
			},
		},
		// Quality presets map agent roles to model tiers.
		// "ultra" uses Opus for all agents; "balanced" is the cost-effective default.
		Quality: QualityConf{
			Default: "balanced",
			Presets: map[string]QualityPreset{
				"ultra": {
					Description: "모든 에이전트를 Opus로 실행. 최고 품질.",
					Agents: map[string]string{
						"planner": "opus", "executor": "opus", "validator": "opus",
						"tester": "opus", "reviewer": "opus", "architect": "opus",
						"spec-writer": "opus", "security-auditor": "opus",
						"debugger": "opus", "explorer": "opus", "devops": "opus",
					},
				},
				"balanced": {
					Description: "핵심 분석은 Opus, 기본 작업은 Sonnet. Haiku 미사용.",
					Agents: map[string]string{
						"planner": "opus", "architect": "opus",
						"spec-writer": "opus", "security-auditor": "opus",
						"executor": "sonnet", "tester": "sonnet",
						"reviewer": "sonnet", "debugger": "sonnet", "devops": "sonnet",
						"validator": "sonnet", "explorer": "sonnet",
					},
				},
			},
		},
		Skills: SkillsConf{
			AutoActivate:    true,
			MaxActiveSkills: 5,
			CategoryWeights: map[string]int{
				"security": 30,
				"quality":  20,
				"agentic":  15,
				"workflow": 10,
			},
		},
		Verify: VerifyConf{
			Enabled:         true,
			DefaultViewport: "desktop",
			AutoFix:         true,
			MaxFixAttempts:  2,
		},
		Design: DesignConf{
			Enabled:         true,
			MaxContextLines: 80,
			InjectOnReview:  true,
			InjectOnVerify:  true,
			ExternalImports: false,
		},
		Context: ContextConf{
			SignatureMap: true,
		},
		Features: FeaturesConf{
			CC21: CC21FeaturesConf{
				Enabled:                 false,
				EffortEnabled:           false,
				MonitorEnabled:          false,
				TaskCreatedEnabled:      false,
				InitialPromptEnabled:    false,
				TaskCreatedMode:         "warn",
				MonitorPatternTimeoutMS: 30000,
			},
		},
	}
}
