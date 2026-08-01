package content

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// contentRuleDir is the embedded rule source directory.
const contentRuleDir = "rules"

// ConditionalDispatcherHooks derives the SPEC-CONDRULE-001 dispatcher entries
// from the compiled rule classification: one entry per distinct hook event and
// matcher pair (REQ-CONDRULE-COMPILE-03).
//
// Deriving rather than hardcoding is load-bearing. A rule scoped to
// `tool:edit(<glob>)` compiles to an Edit|Write|MultiEdit matcher, and this
// registration is the only thing that reaches settings.json. A fixed
// PreToolUse/Bash entry would relocate that rule's body out of baseline context
// and then never fire it, so the rule would silently disappear.
//
// The hook shape itself — type, command, timeout — is owned by pkg/rulecond and
// deliberately not restated here; see the @AX:ANCHOR on the dispatcher constants
// in pkg/rulecond/compile_claude.go. A second copy of those values would drift
// without failing any build.
func ConditionalDispatcherHooks(rules []*rulecond.Rule) ([]adapter.HookConfig, error) {
	compiled, err := rulecond.CompileClaude(rules)
	if err != nil {
		return nil, fmt.Errorf("conditional rule compile: %w", err)
	}
	return compiled.Hooks, nil
}

// appendConditionalDispatcher registers a dispatcher entry for every compiled
// event and matcher pair. Claude Code only: no other platform speaks
// hookSpecificOutput.additionalContext or receives the compiled manifest.
//
// A rule that fails to parse or compile fails generation rather than dropping
// its trigger, which keeps the failure loud instead of producing a harness that
// silently never fires the rule.
func appendConditionalDispatcher(hooks []adapter.HookConfig, platform string) ([]adapter.HookConfig, error) {
	if platform != "claude" && platform != "claude-code" {
		return hooks, nil
	}

	rules, err := embeddedRules()
	if err != nil {
		return nil, err
	}
	dispatchers, err := ConditionalDispatcherHooks(rules)
	if err != nil {
		return nil, err
	}
	for _, dispatcher := range dispatchers {
		hooks = appendUniqueHook(hooks, dispatcher)
	}
	return hooks, nil
}

// embeddedRules parses every rule shipped in the embedded content FS.
func embeddedRules() ([]*rulecond.Rule, error) {
	entries, err := fs.ReadDir(contentfs.FS, contentRuleDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded rules: %w", err)
	}

	rules := make([]*rulecond.Rule, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		srcPath := path.Join(contentRuleDir, entry.Name())
		raw, readErr := fs.ReadFile(contentfs.FS, srcPath)
		if readErr != nil {
			return nil, fmt.Errorf("read rule %s: %w", entry.Name(), readErr)
		}
		rule, parseErr := rulecond.ParseRule(entry.Name(), raw)
		if parseErr != nil {
			return nil, fmt.Errorf("parse rule %s: %w", entry.Name(), parseErr)
		}
		if validateErr := rulecond.Validate(rule); validateErr != nil {
			return nil, fmt.Errorf("invalid rule %s: %w", entry.Name(), validateErr)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}
