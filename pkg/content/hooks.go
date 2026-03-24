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
func generateCLIHooks(cfg config.HooksConf, _ string) []adapter.HookConfig {
	var hooks []adapter.HookConfig

	if cfg.PreCommitArch {
		hooks = append(hooks, adapter.HookConfig{
			Event:   "PreToolUse",
			Matcher: "Bash",
			Type:    "command",
			Command: "auto check --arch --quiet --warn-only",
			Timeout: 30,
		})
	}

	if cfg.PreCommitLore {
		hooks = append(hooks, adapter.HookConfig{
			Event:   "PreToolUse",
			Matcher: "Bash",
			Type:    "command",
			Command: "auto check --lore --quiet",
			Timeout: 30,
		})
	}

	if cfg.ReactCIFailure {
		hooks = append(hooks, adapter.HookConfig{
			Event:   "PostToolUse",
			Matcher: "Bash",
			Type:    "command",
			Command: "auto react --ci-failure --quiet",
			Timeout: 60,
		})
	}

	if cfg.ReactReview {
		hooks = append(hooks, adapter.HookConfig{
			Event:   "PostToolUse",
			Matcher: "Bash",
			Type:    "command",
			Command: "auto react --review --quiet",
			Timeout: 60,
		})
	}

	return hooks
}

// generateGitHooks는 .git/hooks/ 스크립트를 생성한다.
func generateGitHooks(cfg config.HooksConf) []GitHookScript {
	// arch 또는 lore 활성화된 경우에만 pre-commit 생성
	if !cfg.PreCommitArch && !cfg.PreCommitLore {
		return nil
	}

	script := buildPreCommitScript(cfg)
	return []GitHookScript{
		{
			Path:    ".git/hooks/pre-commit",
			Content: script,
		},
	}
}

// buildPreCommitScript는 pre-commit 스크립트를 생성한다.
func buildPreCommitScript(cfg config.HooksConf) string {
	s := "#!/bin/sh\n# Autopus-ADK pre-commit hook (자동 생성)\nset -e\n\n"

	if cfg.PreCommitArch {
		s += "# 아키텍처 규칙 검사\nauto check --arch --quiet\n\n"
	}

	if cfg.PreCommitLore {
		s += "# Lore 커밋 메시지 검사\nauto check --lore --quiet\n\n"
	}

	s += "exit 0\n"
	return s
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
