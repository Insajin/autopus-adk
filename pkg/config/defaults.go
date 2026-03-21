package config

// DefaultFullConfig는 Full 모드 기본 설정을 반환한다.
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
			Providers: map[string]ProviderEntry{
				"claude": {Binary: "claude", Args: []string{"--print"}},
				"codex":  {Binary: "codex", Args: []string{"--quiet"}},
				"gemini": {Binary: "gemini", Args: []string{}, PromptViaArgs: true},
			},
			Commands: map[string]CommandEntry{
				"review": {Strategy: "debate", Providers: []string{"claude", "codex", "gemini"}},
				"plan":   {Strategy: "consensus", Providers: []string{"claude", "gemini"}},
				"secure": {Strategy: "consensus", Providers: []string{"claude", "gemini"}},
			},
		},
	}
}

// DefaultLiteConfig는 Lite 모드 기본 설정을 반환한다.
func DefaultLiteConfig(projectName string) *HarnessConfig {
	return &HarnessConfig{
		Mode:        ModeLite,
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
		},
		Hooks: HooksConf{
			PreCommitArch: true,
		},
		Orchestra: OrchestraConf{
			Enabled: false,
		},
	}
}
