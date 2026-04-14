package codex

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

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
	_ = cfg
	var files []adapter.FileMapping

	entries, err := contentfs.FS.ReadDir("rules")
	if err != nil {
		return nil, fmt.Errorf("codex rule 콘텐츠 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		name := entry.Name()
		raw, err := fs.ReadFile(contentfs.FS, pkgcontent.EmbeddedPath("rules", name))
		if err != nil {
			return nil, fmt.Errorf("codex rule 콘텐츠 읽기 실패 %s: %w", name, err)
		}

		rendered := pkgcontent.ReplacePlatformReferences(string(raw), "codex")
		rendered = ensureCodexRulePlatform(rendered)
		rendered = normalizeCodexInvocationBody(rendered)
		rendered = normalizeCodexHelperPaths(rendered)
		rendered = normalizeCodexToolingBody(rendered)

		relPath := ruleFilePath(name)
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

func ensureCodexRulePlatform(content string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}

	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return content
	}

	frontmatter := rest[:idx]
	body := rest[idx+len("\n---\n"):]
	if !strings.Contains(frontmatter, "\nplatform:") && !strings.HasPrefix(frontmatter, "platform:") {
		frontmatter += "\nplatform: codex"
	}

	return "---\n" + frontmatter + "\n---\n" + body
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
