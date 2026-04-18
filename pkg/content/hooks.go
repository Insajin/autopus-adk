package content

import (
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// GitHookScript는 Git 훅 스크립트이다.
type GitHookScript struct {
	// Path는 훅 파일 경로이다.
	Path string
	// Content는 스크립트 내용이다.
	Content string
}

// GenerateHookConfigs는 설정에 따라 훅 설정을 생성한다.
// supportsHooks가 true이면 CLI 훅 설정을 반환하고,
// false이면 .git/hooks/ 스크립트를 반환한다.
func GenerateHookConfigs(cfg config.HooksConf, platform string, supportsHooks bool) ([]adapter.HookConfig, []GitHookScript, error) {
	if supportsHooks {
		return generateCLIHooks(cfg, platform), nil, nil
	}
	return nil, generateGitHooks(cfg), nil
}

// generateCLIHooks는 CLI 훅 설정을 생성한다.
// Event names are translated per-platform (Claude uses PreToolUse/PostToolUse,
// Gemini uses BeforeTool/AfterTool). Claude is the canonical source.
func generateCLIHooks(cfg config.HooksConf, platform string) []adapter.HookConfig {
	var hooks []adapter.HookConfig
	pre := translateHookEvent("PreToolUse", platform)
	post := translateHookEvent("PostToolUse", platform)

	if cfg.PreCommitArch {
		hooks = appendUniqueHook(hooks, adapter.HookConfig{
			Event:   pre,
			Matcher: "Bash",
			Type:    "command",
			Command: "auto check --arch --quiet --warn-only",
			Timeout: 30,
		})
	}

	// Lore check: not added as PreToolUse hook — it runs via git commit-msg hook only.
	// Checking lore on every Bash call would fail because it validates the last commit,
	// not the current action. The commit-msg hook validates the message being committed.

	if cfg.ReactCIFailure {
		hooks = appendUniqueHook(hooks, adapter.HookConfig{
			Event:   post,
			Matcher: "Bash",
			Type:    "command",
			Command: "auto react check --quiet",
			Timeout: 60,
		})
	}

	if cfg.ReactReview {
		hooks = appendUniqueHook(hooks, adapter.HookConfig{
			Event:   post,
			Matcher: "Bash",
			Type:    "command",
			Command: "auto react check --quiet",
			Timeout: 60,
		})
	}

	return hooks
}

// translateHookEvent maps Claude Code event names to the platform's native
// event names. Unknown platforms pass through the Claude Code name unchanged
// (safe default — most adapters will reject unknown names and log a warning).
func translateHookEvent(claudeEvent, platform string) string {
	if platform != "gemini" && platform != "gemini-cli" {
		return claudeEvent
	}
	switch claudeEvent {
	case "PreToolUse":
		return "BeforeTool"
	case "PostToolUse":
		return "AfterTool"
	default:
		return claudeEvent
	}
}

func appendUniqueHook(hooks []adapter.HookConfig, hook adapter.HookConfig) []adapter.HookConfig {
	for _, existing := range hooks {
		if existing == hook {
			return hooks
		}
	}
	return append(hooks, hook)
}

// generateGitHooks는 .git/hooks/ 스크립트를 생성한다.
// pre-commit: arch check with --staged (only staged files).
// commit-msg: lore format check on the commit message being written.
func generateGitHooks(cfg config.HooksConf) []GitHookScript {
	var hooks []GitHookScript

	if cfg.PreCommitArch {
		hooks = append(hooks, GitHookScript{
			Path:    ".git/hooks/pre-commit",
			Content: buildPreCommitScript(cfg),
		})
	}

	if cfg.PreCommitLore {
		hooks = append(hooks, GitHookScript{
			Path:    ".git/hooks/commit-msg",
			Content: buildCommitMsgScript(),
		})
	}

	return hooks
}

// buildPreCommitScript는 pre-commit 스크립트를 생성한다.
// Uses --staged to only check git-staged files, avoiding submodule/worktree scans.
func buildPreCommitScript(cfg config.HooksConf) string {
	s := "#!/bin/sh\n# Autopus-ADK pre-commit hook (자동 생성)\nset -e\n\n"

	if cfg.PreCommitArch {
		s += "# 아키텍처 규칙 검사 (staged 파일만)\nauto check --arch --quiet --staged\n\n"
	}

	s += "exit 0\n"
	return s
}

// buildCommitMsgScript는 commit-msg 스크립트를 생성한다.
// The commit message file path is passed as $1 by git.
func buildCommitMsgScript() string {
	return "#!/bin/sh\n# Autopus-ADK commit-msg hook (자동 생성)\nset -e\n\n" +
		"ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)\ncd \"$ROOT\"\n\n" +
		"# Lore 커밋 메시지 검사\nauto check --lore --quiet --message \"$1\"\n" +
		"auto lore validate \"$1\"\n\n" +
		"exit 0\n"
}

// DetectPermissions는 프로젝트 루트를 분석하여 기본 권한을 생성한다.
func DetectPermissions(projectRoot string, extra config.PermissionsConf) *adapter.PermissionSet {
	allow := []string{
		// Common: always included
		"Bash(auto *)",
		"Bash(auto:*)",
		"Bash(git *)",
		"Bash(git:*)",
		"Bash(make:*)",
		"Bash(ls:*)",
		"Bash(cat:*)",
		"Bash(find:*)",
		"Bash(grep:*)",
		"Bash(wc:*)",
		"Bash(sort:*)",
		"Bash(mkdir:*)",
		"Bash(echo:*)",
		"Bash(gh:*)",
		"mcp__sequential-thinking__sequentialthinking",
		"WebSearch",

		// Pipeline: agent orchestration tools
		"Agent",
		"AskUserQuestion",
		"TaskCreate",
		"TaskUpdate",
		"TeamCreate",
		"SendMessage",
		"ToolSearch",
	}

	// Go project detection
	if fileExists(filepath.Join(projectRoot, "go.mod")) {
		allow = append(allow,
			"Bash(go build:*)", "Bash(go test:*)", "Bash(go vet:*)",
			"Bash(go run:*)", "Bash(go mod:*)", "Bash(go tool:*)",
			"Bash(go get:*)", "Bash(go install:*)", "Bash(go version:*)",
			"Bash(go env:*)", "Bash(go clean:*)",
			"Bash(golangci-lint:*)", "Bash(gofmt:*)",
		)
	}

	// Node project detection
	if fileExists(filepath.Join(projectRoot, "package.json")) {
		allow = append(allow,
			"Bash(npm *)", "Bash(npx *)", "Bash(node *)",
			"Bash(pnpm *)", "Bash(yarn *)",
		)
	}

	// Merge extra permissions from autopus.yaml
	allow = append(allow, extra.ExtraAllow...)

	var deny []string
	deny = append(deny, extra.ExtraDeny...)

	return &adapter.PermissionSet{
		Allow: allow,
		Deny:  deny,
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
