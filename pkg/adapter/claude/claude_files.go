package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/templates"
)

// prepareFiles는 Generate와 동일한 파일을 준비하되, 디스크에 쓰지 않고 내용만 반환한다.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	// CLAUDE.md marker section
	claudeMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("CLAUDE.md 마커 주입 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      "CLAUDE.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        checksum(claudeMD),
		Content:         []byte(claudeMD),
	})

	// Router command
	tmplContent, err := templates.FS.ReadFile("claude/commands/auto-router.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("라우터 템플릿 읽기 실패: %w", err)
	}
	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("라우터 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      filepath.Join(".claude", "commands", "auto.md"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	})

	// .mcp.json
	mcpFiles, err := a.prepareMCPConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("MCP 설정 준비 실패: %w", err)
	}
	files = append(files, mcpFiles...)

	// Statusline script
	statusFiles, err := a.prepareStatusline()
	if err != nil {
		return nil, fmt.Errorf("statusline 준비 실패: %w", err)
	}
	files = append(files, statusFiles...)

	// Hooks
	hookFiles, err := a.prepareContentFiles("hooks", filepath.Join(".claude", "hooks", "autopus"))
	if err != nil {
		return nil, fmt.Errorf("훅 파일 준비 실패: %w", err)
	}
	files = append(files, hookFiles...)

	// Rules content files (all modes)
	ruleFiles, err := a.prepareContentFiles("rules", filepath.Join(".claude", "rules", "autopus"))
	if err != nil {
		return nil, fmt.Errorf("룰 파일 준비 실패: %w", err)
	}
	files = append(files, ruleFiles...)

	// Render file-size-limit.md from template (stack/framework-aware)
	fileSizeRule, err := a.prepareFileSizeLimitRule(cfg)
	if err != nil {
		return nil, fmt.Errorf("file-size-limit 룰 준비 실패: %w", err)
	}
	files = append(files, fileSizeRule)

	// Full mode: skills/agents
	if cfg.IsFullMode() {
		skillFiles, err := a.prepareContentFiles("skills", ".claude/skills/autopus")
		if err != nil {
			return nil, fmt.Errorf("스킬 파일 준비 실패: %w", err)
		}
		files = append(files, skillFiles...)

		agentFiles, err := a.prepareContentFiles("agents", ".claude/agents/autopus")
		if err != nil {
			return nil, fmt.Errorf("에이전트 파일 준비 실패: %w", err)
		}
		files = append(files, agentFiles...)
	}

	return files, nil
}

// prepareContentFiles는 컨텐츠 파일을 읽어 FileMapping 슬라이스로 반환한다 (디스크 쓰기 없음).
// file-size-limit.md is skipped here; it is rendered dynamically via prepareFileSizeLimitRule.
func (a *Adapter) prepareContentFiles(subDir string, targetRelDir string) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := contentfs.FS.ReadDir(subDir)
	if err != nil {
		return nil, fmt.Errorf("컨텐츠 디렉터리 읽기 실패 %s: %w", subDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// file-size-limit.md is rendered from template — skip the static copy.
		if subDir == "rules" && entry.Name() == "file-size-limit.md" {
			continue
		}
		srcPath := subDir + "/" + entry.Name()
		data, err := fs.ReadFile(contentfs.FS, srcPath)
		if err != nil {
			return nil, fmt.Errorf("컨텐츠 파일 읽기 실패 %s: %w", srcPath, err)
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(targetRelDir, entry.Name()),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(string(data)),
			Content:         data,
		})
	}
	return files, nil
}

// prepareMCPConfig는 .mcp.json 내용을 준비한다 (디스크 쓰기 없음).
func (a *Adapter) prepareMCPConfig(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("claude/mcp.json.tmpl")
	if err != nil {
		return nil, fmt.Errorf("MCP 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("MCP 템플릿 렌더링 실패: %w", err)
	}

	// Parse rendered JSON
	var newMCP map[string]interface{}
	if err := json.Unmarshal([]byte(rendered), &newMCP); err != nil {
		return nil, fmt.Errorf("MCP JSON 파싱 실패: %w", err)
	}

	// Merge with existing .mcp.json to preserve user servers
	targetPath := filepath.Join(a.root, ".mcp.json")
	if data, err := os.ReadFile(targetPath); err == nil {
		var existing map[string]interface{}
		if err := json.Unmarshal(data, &existing); err == nil {
			existingServers, _ := existing["mcpServers"].(map[string]interface{})
			newServers, _ := newMCP["mcpServers"].(map[string]interface{})
			if existingServers != nil && newServers != nil {
				for k, v := range newServers {
					existingServers[k] = v
				}
				existing["mcpServers"] = existingServers
				newMCP = existing
			}
		}
	}

	out, err := json.MarshalIndent(newMCP, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("MCP JSON 직렬화 실패: %w", err)
	}
	outStr := string(out) + "\n"

	return []adapter.FileMapping{{
		TargetPath:      ".mcp.json",
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(outStr),
		Content:         []byte(outStr),
	}}, nil
}

// prepareStatusline reads statusline.sh from embedded FS and returns a FileMapping.
func (a *Adapter) prepareStatusline() ([]adapter.FileMapping, error) {
	data, err := contentfs.FS.ReadFile("statusline.sh")
	if err != nil {
		return nil, fmt.Errorf("statusline.sh 읽기 실패: %w", err)
	}
	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".claude", "statusline.sh"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(string(data)),
		Content:         data,
	}}, nil
}

// copyStatusline copies statusline.sh to the target project.
func (a *Adapter) copyStatusline() ([]adapter.FileMapping, error) {
	data, err := contentfs.FS.ReadFile("statusline.sh")
	if err != nil {
		return nil, fmt.Errorf("statusline.sh 읽기 실패: %w", err)
	}
	destPath := filepath.Join(a.root, ".claude", "statusline.sh")
	if err := os.WriteFile(destPath, data, 0755); err != nil {
		return nil, fmt.Errorf("statusline.sh 쓰기 실패: %w", err)
	}
	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".claude", "statusline.sh"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(string(data)),
		Content:         data,
	}}, nil
}

// renderRouterCommand는 단일 라우터 템플릿(auto-router.md.tmpl)을 렌더링하여
// .claude/commands/auto.md 파일을 생성한다.
func (a *Adapter) renderRouterCommand(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("claude/commands/auto-router.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("라우터 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("라우터 템플릿 렌더링 실패: %w", err)
	}

	targetPath := filepath.Join(a.root, ".claude", "commands", "auto.md")
	if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
		return nil, fmt.Errorf("라우터 커맨드 쓰기 실패: %w", err)
	}

	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".claude", "commands", "auto.md"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(rendered),
		Content:         []byte(rendered),
	}}, nil
}

// copyContentFiles는 embedded content FS에서 파일을 읽어 대상 디렉터리에 복사한다.
// subDir: "skills" or "agents"
// targetRelDir: relative destination path (e.g. ".claude/skills/autopus")
// file-size-limit.md in "rules" is skipped here and rendered dynamically instead.
func (a *Adapter) copyContentFiles(cfg *config.HarnessConfig, subDir string, targetRelDir string) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := contentfs.FS.ReadDir(subDir)
	if err != nil {
		return nil, fmt.Errorf("컨텐츠 디렉터리 읽기 실패 %s: %w", subDir, err)
	}

	// Create destination directory
	absTargetDir := filepath.Join(a.root, targetRelDir)
	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		return nil, fmt.Errorf("대상 디렉터리 생성 실패 %s: %w", absTargetDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// file-size-limit.md is rendered from template — skip the static copy.
		if subDir == "rules" && entry.Name() == "file-size-limit.md" {
			continue
		}

		srcPath := subDir + "/" + entry.Name()
		data, err := fs.ReadFile(contentfs.FS, srcPath)
		if err != nil {
			return nil, fmt.Errorf("컨텐츠 파일 읽기 실패 %s: %w", srcPath, err)
		}

		destPath := filepath.Join(absTargetDir, entry.Name())
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return nil, fmt.Errorf("컨텐츠 파일 쓰기 실패 %s: %w", destPath, err)
		}

		relPath := filepath.Join(targetRelDir, entry.Name())
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(string(data)),
			Content:         data,
		})
	}

	return files, nil
}
