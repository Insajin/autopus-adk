package config

// DefaultFullConfig returns the default config for Full mode.
// @AX:NOTE: [AUTO] magic constants — model names (opus, sonnet, haiku, gpt-5.4, gemini-3.1-pro-preview), timeouts, and tier mappings are hardcoded below
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
				Enabled:            true,
				Strategy:           "debate",
				Providers:          []string{"claude", "gemini"},
				Judge:              "claude",
				MaxRevisions:       2,
				AutoCollectContext: true,
				ContextMaxLines:    500,
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
				"premium":  "claude-opus-4-6",
				"standard": "claude-sonnet-4-6",
				"economy":  "claude-haiku-4-5",
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
			PreCommitArch: true,
		},
		Session: SessionConf{
			HandoffEnabled:   true,
			ContinueFile:     ".auto-continue.md",
			MaxContextTokens: 2000,
		},
		Orchestra: OrchestraConf{
			Enabled:         true,
			DefaultStrategy: "consensus",
			TimeoutSeconds:  120,
			Judge:           "claude",
			Providers: map[string]ProviderEntry{
				"claude":   {Binary: "claude", Args: []string{"--print"}, PaneArgs: []string{"--print"}},
				"gemini":   {Binary: "gemini", Args: []string{}, PaneArgs: []string{}, PromptViaArgs: true},
				"codex": {Binary: "codex", Args: []string{"exec", "--full-auto", "-m", "gpt-5.4"}, PaneArgs: []string{"-m", "gpt-5.4"}, PromptViaArgs: false},
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
					Description: "핵심 분석은 Opus, 구현은 Sonnet, 검증은 Haiku. 가성비 최적.",
					Agents: map[string]string{
						"planner": "opus", "architect": "opus",
						"spec-writer": "opus", "security-auditor": "opus",
						"executor": "sonnet", "tester": "sonnet",
						"reviewer": "sonnet", "debugger": "sonnet", "devops": "sonnet",
						"validator": "haiku", "explorer": "haiku",
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
		Context: ContextConf{
			SignatureMap: true,
		},
	}
}
