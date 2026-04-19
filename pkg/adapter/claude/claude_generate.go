package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

// Generate는 하네스 설정에 기반하여 Claude Code 파일을 생성한다.
func (a *Adapter) Generate(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// Create required directories.
	dirs := []string{
		filepath.Join(a.root, ".claude", "rules", "autopus"),
		filepath.Join(a.root, ".claude", "skills", "autopus"),
		filepath.Join(a.root, ".claude", "commands"),
		filepath.Join(a.root, ".claude", "agents", "autopus"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패 %s: %w", d, err)
		}
	}

	// Clean up legacy command surfaces during harness upgrades.
	legacyCmdDir := filepath.Join(a.root, ".claude", "commands", "autopus")
	if _, err := os.Stat(legacyCmdDir); err == nil {
		if err := os.RemoveAll(legacyCmdDir); err != nil {
			return nil, fmt.Errorf("레거시 커맨드 디렉터리 정리 실패 %s: %w", legacyCmdDir, err)
		}
	}

	legacyAutoMD := filepath.Join(a.root, ".claude", "commands", "auto.md")
	if _, err := os.Stat(legacyAutoMD); err == nil {
		_ = os.Remove(legacyAutoMD)
	}

	var files []adapter.FileMapping

	claudeMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("CLAUDE.md 마커 주입 실패: %w", err)
	}

	claudePath := filepath.Join(a.root, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(claudeMD), 0644); err != nil {
		return nil, fmt.Errorf("CLAUDE.md 쓰기 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      "CLAUDE.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        checksum(claudeMD),
		Content:         []byte(claudeMD),
	})

	commandFiles, err := a.renderRouterCommand(cfg)
	if err != nil {
		return nil, fmt.Errorf("커맨드 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, commandFiles...)

	mcpFiles, err := a.prepareMCPConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("MCP 설정 생성 실패: %w", err)
	}
	for _, f := range mcpFiles {
		targetPath := filepath.Join(a.root, f.TargetPath)
		if err := os.WriteFile(targetPath, f.Content, 0644); err != nil {
			return nil, fmt.Errorf(".mcp.json 쓰기 실패: %w", err)
		}
	}
	files = append(files, mcpFiles...)

	if err := a.applyHooksAndPermissions(ctx, cfg); err != nil {
		return nil, err
	}

	ruleFiles, err := a.copyContentFiles(cfg, "rules", filepath.Join(".claude", "rules", "autopus"))
	if err != nil {
		return nil, fmt.Errorf("룰 파일 복사 실패: %w", err)
	}
	files = append(files, ruleFiles...)

	fileSizeRule, err := a.prepareFileSizeLimitRule(cfg)
	if err != nil {
		return nil, fmt.Errorf("file-size-limit 룰 생성 실패: %w", err)
	}
	destPath := filepath.Join(a.root, fileSizeRule.TargetPath)
	if err := os.WriteFile(destPath, fileSizeRule.Content, 0644); err != nil {
		return nil, fmt.Errorf("file-size-limit.md 쓰기 실패: %w", err)
	}
	files = append(files, fileSizeRule)

	statusFiles, err := a.copyStatusline()
	if err != nil {
		return nil, fmt.Errorf("statusline 복사 실패: %w", err)
	}
	files = append(files, statusFiles...)

	// Managed hook assets are split between autopus/ and root hook files.
	hookFiles, err := a.copyContentFiles(cfg, "hooks", filepath.Join(".claude", "hooks", "autopus"))
	if err != nil {
		return nil, fmt.Errorf("훅 파일 복사 실패: %w", err)
	}
	files = append(files, hookFiles...)

	rootHookFiles, err := a.copyNamedContentFiles("hooks", filepath.Join(".claude", "hooks"), claudeRootHookFiles)
	if err != nil {
		return nil, fmt.Errorf("CC21 루트 훅 파일 복사 실패: %w", err)
	}
	files = append(files, rootHookFiles...)

	if cfg.IsFullMode() {
		skillFiles, err := a.copyContentFiles(cfg, "skills", ".claude/skills/autopus")
		if err != nil {
			return nil, fmt.Errorf("스킬 파일 복사 실패: %w", err)
		}
		files = append(files, skillFiles...)

		agentFiles, err := a.copyContentFiles(cfg, "agents", ".claude/agents/autopus")
		if err != nil {
			return nil, fmt.Errorf("에이전트 파일 복사 실패: %w", err)
		}
		files = append(files, agentFiles...)
	}

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(claudeMD),
	}

	m := adapter.ManifestFromFiles(adapterName, pf)
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}

	return pf, nil
}
