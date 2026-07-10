package content

import (
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// --- agent_transformer helpers ---

func TestEscapeTOMLBasicMultiline(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"line1\nline2", "line1\nline2"},
		// backslash escaping
		{`a\b`, `a\\b`},
		// triple-quote escaping
		{`before"""after`, `before\"\"\"after`},
		// leading/trailing whitespace stripped
		{"  hello  ", "hello"},
		// CRLF normalized to LF
		{"line1\r\nline2", "line1\nline2"},
	}
	for _, tc := range cases {
		got := escapeTOMLBasicMultiline(tc.in)
		if got != tc.want {
			t.Errorf("escapeTOMLBasicMultiline(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatTOMLMultilineString(t *testing.T) {
	// No single-quotes triple: use literal string block.
	basic := formatTOMLMultilineString("hello world")
	if !strings.HasPrefix(basic, "'''\n") || !strings.HasSuffix(basic, "\n'''") {
		t.Errorf("basic TOML literal = %q", basic)
	}
	if !strings.Contains(basic, "hello world") {
		t.Errorf("basic content missing: %q", basic)
	}
	// Contains triple single-quotes: fall back to basic string block.
	withTriple := formatTOMLMultilineString("a'''b")
	if !strings.HasPrefix(withTriple, "\"\"\"\n") || !strings.HasSuffix(withTriple, "\n\"\"\"") {
		t.Errorf("fallback TOML basic = %q", withTriple)
	}
}

// --- hooks helpers ---

func TestSameHookConfig(t *testing.T) {
	base := adapter.HookConfig{
		Event: "PreToolUse", Matcher: "Bash", Type: "command",
		Command: "auto check", Timeout: 30,
		Env: map[string]string{"K": "V"},
	}
	clone := base
	if !sameHookConfig(base, clone) {
		t.Error("identical hooks must be equal")
	}
	// Different event.
	diffEvent := base
	diffEvent.Event = "PostToolUse"
	if sameHookConfig(base, diffEvent) {
		t.Error("different event must differ")
	}
	// Different env value.
	diffEnv := base
	diffEnv.Env = map[string]string{"K": "OTHER"}
	if sameHookConfig(base, diffEnv) {
		t.Error("different env value must differ")
	}
	// Different env length.
	extraEnv := base
	extraEnv.Env = map[string]string{"K": "V", "X": "Y"}
	if sameHookConfig(base, extraEnv) {
		t.Error("different env length must differ")
	}
	// Nil env vs empty env: both have len 0 → equal.
	noEnvA := adapter.HookConfig{Event: "E", Matcher: "B", Type: "command", Command: "c", Timeout: 1}
	noEnvB := noEnvA
	noEnvB.Env = map[string]string{}
	if !sameHookConfig(noEnvA, noEnvB) {
		t.Error("nil vs empty env must be equal")
	}
}

func TestGenerateProjectHookConfigs_NilCfg(t *testing.T) {
	hooks, gitHooks, err := GenerateProjectHookConfigs(nil, "claude-code", true)
	if err != nil {
		t.Fatalf("nil cfg: %v", err)
	}
	// nil cfg with supportsHooks=true: GenerateHookConfigs with empty HooksConf → no hooks.
	_ = hooks
	_ = gitHooks
}

func TestGenerateProjectHookConfigs_GitHooksFallback(t *testing.T) {
	cfg := &config.HarnessConfig{}
	cfg.Hooks.PreCommitArch = true
	cfg.Hooks.PreCommitLore = true
	_, gitHooks, err := GenerateProjectHookConfigs(cfg, "claude-code", false)
	if err != nil {
		t.Fatalf("git hooks: %v", err)
	}
	paths := make(map[string]bool, len(gitHooks))
	for _, gh := range gitHooks {
		paths[gh.Path] = true
	}
	if !paths[".git/hooks/pre-commit"] {
		t.Error("pre-commit hook missing")
	}
	if !paths[".git/hooks/commit-msg"] {
		t.Error("commit-msg hook missing")
	}
}

// --- skill_transformer_refs ---

func TestRewriteCanonicalSkillReferences(t *testing.T) {
	body := "See .claude/skills/autopus/planning.md for planning."
	// Nil resolver: body unchanged.
	got := rewriteCanonicalSkillReferences(body, nil)
	if got != body {
		t.Errorf("nil resolver changed body: %q", got)
	}
	// Resolver maps the skill name to a new path.
	got = rewriteCanonicalSkillReferences(body, func(name string) string {
		if name == "planning" {
			return ".codex/skills/planning.md"
		}
		return ""
	})
	if !strings.Contains(got, ".codex/skills/planning.md") {
		t.Errorf("rewrite did not replace: %q", got)
	}
	if strings.Contains(got, ".claude/skills/autopus/planning.md") {
		t.Errorf("original ref still present: %q", got)
	}
	// Resolver returns empty → original ref preserved.
	got = rewriteCanonicalSkillReferences(body, func(_ string) string { return "" })
	if got != body {
		t.Errorf("empty-resolver changed body: %q", got)
	}
}

// --- generate helpers ---

func TestValidateName(t *testing.T) {
	if err := validateName("valid-name"); err != nil {
		t.Errorf("valid name error: %v", err)
	}
	cases := []string{"", "path/sep", "back\\slash", "dot..dot"}
	for _, bad := range cases {
		if err := validateName(bad); err == nil {
			t.Errorf("validateName(%q) = nil, want error", bad)
		}
	}
}

// --- router helpers ---

func TestTierToModel(t *testing.T) {
	tiers := map[string]string{"fast": "gpt-4o-mini", "smart": "o3"}
	if got := tierToModel(tiers, "fast"); got != "gpt-4o-mini" {
		t.Errorf("tierToModel(fast) = %q", got)
	}
	// Unknown tier returns the tier name itself.
	if got := tierToModel(tiers, "unknown"); got != "unknown" {
		t.Errorf("tierToModel(unknown) = %q", got)
	}
}

func TestIsKnownCategory(t *testing.T) {
	for _, known := range []string{"visual", "deep", "quick", "ultrabrain", "writing", "git", "adaptive"} {
		if !isKnownCategory(known) {
			t.Errorf("isKnownCategory(%q) = false, want true", known)
		}
	}
	if isKnownCategory("nonexistent") {
		t.Error("isKnownCategory(nonexistent) = true, want false")
	}
}
