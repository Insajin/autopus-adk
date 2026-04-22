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

var claudeRootHookFiles = []string{
	"task-created-validate.sh",
	"README.md",
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

// renderRouterCommand는 단일 라우터 템플릿(auto-router.md.tmpl)을 렌더링하여
// .claude/skills/auto/SKILL.md 파일을 생성한다.
func (a *Adapter) renderRouterCommand(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	tmplContent, err := templates.FS.ReadFile("claude/commands/auto-router.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("라우터 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return nil, fmt.Errorf("라우터 템플릿 렌더링 실패: %w", err)
	}

	targetDir := filepath.Join(a.root, ".claude", "skills", "auto")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, fmt.Errorf("라우터 스킬 디렉터리 생성 실패: %w", err)
	}

	targetPath := filepath.Join(targetDir, "SKILL.md")
	if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
		return nil, fmt.Errorf("라우터 스킬 쓰기 실패: %w", err)
	}

	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".claude", "skills", "auto", "SKILL.md"),
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
		if subDir == "hooks" && isClaudeRootHookFile(entry.Name()) {
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
		data = normalizeClaudeContent(subDir, data)

		destPath := filepath.Join(absTargetDir, entry.Name())
		if err := os.WriteFile(destPath, data, contentFileMode(entry.Name())); err != nil {
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

func (a *Adapter) copyNamedContentFiles(subDir string, targetRelDir string, names []string) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	absTargetDir := filepath.Join(a.root, targetRelDir)
	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		return nil, fmt.Errorf("대상 디렉터리 생성 실패 %s: %w", absTargetDir, err)
	}

	for _, name := range names {
		srcPath := subDir + "/" + name
		data, err := fs.ReadFile(contentfs.FS, srcPath)
		if err != nil {
			return nil, fmt.Errorf("컨텐츠 파일 읽기 실패 %s: %w", srcPath, err)
		}

		destPath := filepath.Join(absTargetDir, name)
		if err := os.WriteFile(destPath, data, contentFileMode(name)); err != nil {
			return nil, fmt.Errorf("컨텐츠 파일 쓰기 실패 %s: %w", destPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(targetRelDir, name),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(string(data)),
			Content:         data,
		})
	}

	return files, nil
}

func isClaudeRootHookFile(name string) bool {
	for _, candidate := range claudeRootHookFiles {
		if candidate == name {
			return true
		}
	}
	return false
}

func contentFileMode(name string) os.FileMode {
	if filepath.Ext(name) == ".sh" {
		return 0755
	}
	return 0644
}
