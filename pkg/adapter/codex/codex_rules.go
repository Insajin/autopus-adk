package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

const codexRulesTemplateDir = "codex/rules/autopus"

// fileSizeLimitData is the template data for the file-size-limit rule.
type fileSizeLimitData struct {
	Exclusions []content.FileSizeExclusion
}

// generateRuleFiles reads Codex rule templates from embedded FS,
// renders them, and writes to .codex/rules/autopus/.
func (a *Adapter) generateRuleFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings, err := a.prepareRuleMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("codex rules 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("codex rule 파일 쓰기 실패 %s: %w", destPath, err)
		}
	}

	return mappings, nil
}

// prepareRuleMappings renders rule templates and returns file mappings
// without writing to disk.
func (a *Adapter) prepareRuleMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir(codexRulesTemplateDir)
	if err != nil {
		return nil, fmt.Errorf("codex rule 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}

		name := entry.Name()
		outFile := strings.TrimSuffix(name, ".tmpl")

		tmplContent, err := templates.FS.ReadFile(codexRulesTemplateDir + "/" + name)
		if err != nil {
			return nil, fmt.Errorf("codex rule 템플릿 읽기 실패 %s: %w", name, err)
		}

		// file-size-limit uses a special data struct with exclusions.
		var rendered string
		if outFile == "file-size-limit.md" {
			exclusions := content.FileSizeExclusions(cfg.Stack, cfg.Framework)
			data := fileSizeLimitData{Exclusions: exclusions}
			rendered, err = a.engine.RenderString(string(tmplContent), data)
		} else {
			rendered, err = a.engine.RenderString(string(tmplContent), cfg)
		}
		if err != nil {
			return nil, fmt.Errorf("codex rule 템플릿 렌더링 실패 %s: %w", name, err)
		}

		relPath := ruleFilePath(outFile)
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// ruleFilePath returns the target path for a rule file.
// Uses subdirectory mode: .codex/rules/autopus/{name}.
func ruleFilePath(name string) string {
	if detectCodexSubdirSupport() {
		return filepath.Join(".codex", "rules", "autopus", name)
	}
	// Flat fallback: .codex/rules-autopus-{name}
	return filepath.Join(".codex", "rules-autopus-"+name)
}

// detectCodexSubdirSupport checks whether Codex supports subdirectories
// in the rules directory. Defaults to true (subdirectory mode).
//
// Codex CLI does not auto-load files from arbitrary .codex/ subdirectories.
// It reads AGENTS.md as its system prompt and .codex/agents/*.toml for agents.
// Rule files in .codex/rules/autopus/ are referenced from AGENTS.md so the
// model knows to consult them. Subdirectory mode is preferred for cleaner
// organization; flat mode (.codex/rules-autopus-{name}) is the fallback.
// Verified: T5 / SPEC-PARITY-001.
func detectCodexSubdirSupport() bool {
	return true
}

// stripFrontmatter removes YAML frontmatter (--- ... ---) from content.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content
	}
	body := rest[idx+4:]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return body
}
