package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
	"github.com/insajin/autopus-adk/templates"
)

type standardSkillSpec struct {
	Name         string
	Description  string
	TemplatePath string
}

var codexUserFacingSkills = []standardSkillSpec{
	{
		Name:         "auto-plan",
		Description:  "Autopus SPEC 작성 워크플로우. 기능 설명을 분석하고 SPEC 문서를 생성할 때 사용합니다.",
		TemplatePath: "codex/skills/auto-plan.md.tmpl",
	},
	{
		Name:         "auto-go",
		Description:  "Autopus SPEC 구현 워크플로우. TDD 기반으로 SPEC을 구현할 때 사용합니다.",
		TemplatePath: "codex/skills/auto-go.md.tmpl",
	},
	{
		Name:         "auto-fix",
		Description:  "Autopus 버그 수정 워크플로우. 재현 테스트를 먼저 작성하고 최소 수정으로 버그를 해결할 때 사용합니다.",
		TemplatePath: "codex/skills/auto-fix.md.tmpl",
	},
	{
		Name:         "auto-review",
		Description:  "Autopus 코드 리뷰 워크플로우. TRUST 5 기준으로 변경사항을 검토할 때 사용합니다.",
		TemplatePath: "codex/skills/auto-review.md.tmpl",
	},
	{
		Name:         "auto-sync",
		Description:  "Autopus 문서 동기화 워크플로우. 구현 이후 SPEC, CHANGELOG, 문서를 동기화할 때 사용합니다.",
		TemplatePath: "codex/skills/auto-sync.md.tmpl",
	},
	{
		Name:         "auto-idea",
		Description:  "Autopus 아이디어 워크플로우. 멀티 프로바이더 토론과 ICE 평가로 아이디어를 정리할 때 사용합니다.",
		TemplatePath: "codex/skills/auto-idea.md.tmpl",
	},
	{
		Name:         "auto-canary",
		Description:  "Autopus 배포 검증 워크플로우. build, E2E, 브라우저 건강 검진을 실행할 때 사용합니다.",
		TemplatePath: "codex/prompts/auto-canary.md.tmpl",
	},
}

type pluginManifest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Author      pluginAuthor    `json:"author"`
	Homepage    string          `json:"homepage"`
	Repository  string          `json:"repository"`
	License     string          `json:"license"`
	Keywords    []string        `json:"keywords"`
	Skills      string          `json:"skills"`
	Interface   pluginInterface `json:"interface"`
}

type pluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	URL   string `json:"url"`
}

type pluginInterface struct {
	DisplayName       string   `json:"displayName"`
	ShortDescription  string   `json:"shortDescription"`
	LongDescription   string   `json:"longDescription"`
	DeveloperName     string   `json:"developerName"`
	Category          string   `json:"category"`
	Capabilities      []string `json:"capabilities"`
	WebsiteURL        string   `json:"websiteURL"`
	PrivacyPolicyURL  string   `json:"privacyPolicyURL"`
	TermsOfServiceURL string   `json:"termsOfServiceURL"`
	DefaultPrompt     []string `json:"defaultPrompt"`
	BrandColor        string   `json:"brandColor"`
}

type marketplaceDoc struct {
	Name      string             `json:"name"`
	Interface marketplaceUI      `json:"interface,omitempty"`
	Plugins   []marketplaceEntry `json:"plugins"`
}

type marketplaceUI struct {
	DisplayName string `json:"displayName,omitempty"`
}

type marketplaceEntry struct {
	Name     string            `json:"name"`
	Source   marketplaceSource `json:"source"`
	Policy   marketplacePolicy `json:"policy"`
	Category string            `json:"category"`
}

type marketplaceSource struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type marketplacePolicy struct {
	Installation   string   `json:"installation"`
	Authentication string   `json:"authentication"`
	Products       []string `json:"products,omitempty"`
}

func (a *Adapter) renderStandardSkills(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings, err := a.prepareStandardSkillMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("codex standard skill dir 생성 실패 %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("codex standard skill 쓰기 실패 %s: %w", destPath, err)
		}
	}

	return mappings, nil
}

func (a *Adapter) prepareStandardSkillMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	routerContent, err := a.renderRouterSkill(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, newSkillMapping(filepath.Join(".agents", "skills", "auto", "SKILL.md"), routerContent))

	for _, spec := range codexUserFacingSkills {
		content, err := a.renderTemplateAsSkill(cfg, spec)
		if err != nil {
			return nil, err
		}
		files = append(files, newSkillMapping(filepath.Join(".agents", "skills", spec.Name, "SKILL.md"), content))
	}

	return files, nil
}

func (a *Adapter) renderPluginFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	mappings, err := a.preparePluginMappings(cfg)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		destPath := filepath.Join(a.root, m.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("codex plugin dir 생성 실패 %s: %w", filepath.Dir(destPath), err)
		}
		if err := os.WriteFile(destPath, m.Content, 0644); err != nil {
			return nil, fmt.Errorf("codex plugin 파일 쓰기 실패 %s: %w", destPath, err)
		}
	}

	return mappings, nil
}

func (a *Adapter) preparePluginMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	routerContent, err := a.renderRouterSkill(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, newSkillMapping(filepath.Join(".autopus", "plugins", "auto", "skills", "auto", "SKILL.md"), routerContent))

	for _, spec := range codexUserFacingSkills {
		content, err := a.renderTemplateAsSkill(cfg, spec)
		if err != nil {
			return nil, err
		}
		files = append(files, newSkillMapping(filepath.Join(".autopus", "plugins", "auto", "skills", spec.Name, "SKILL.md"), content))
	}

	pluginJSON, err := a.renderPluginManifestJSON()
	if err != nil {
		return nil, err
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      filepath.Join(".autopus", "plugins", "auto", ".codex-plugin", "plugin.json"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(pluginJSON),
		Content:         []byte(pluginJSON),
	})

	marketplaceJSON, err := a.renderMarketplaceJSON()
	if err != nil {
		return nil, err
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      filepath.Join(".agents", "plugins", "marketplace.json"),
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(marketplaceJSON),
		Content:         []byte(marketplaceJSON),
	})

	return files, nil
}

func (a *Adapter) renderRouterSkill(cfg *config.HarnessConfig) (string, error) {
	tmplContent, err := templates.FS.ReadFile("claude/commands/auto-router.md.tmpl")
	if err != nil {
		return "", fmt.Errorf("codex router skill 템플릿 읽기 실패: %w", err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return "", fmt.Errorf("codex router skill 템플릿 렌더링 실패: %w", err)
	}

	frontmatter, body := splitSkillFrontmatter(rendered)
	body = pkgcontent.ReplacePlatformReferences(body, "codex")
	body = strings.Replace(body, "# /auto — Autopus-ADK", "# auto — Autopus-ADK", 1)
	body = strings.Replace(body, "ARGUMENTS: $ARGUMENTS", "Interpret the user's text after the explicit `auto` mention as the command arguments. Support both `@auto ...` via the local Codex plugin and `$auto ...` via the repository skill.", 1)
	body = normalizeCodexInvocationBody(body)

	aliasNote := strings.TrimSpace(fmt.Sprintf(`
## Codex Invocation

Use this skill through either of these surfaces:

- %s — when the local Autopus plugin is installed from %s
- %s — when using repository-scoped skills only

Throughout this document, prefer %s. If the plugin is not installed, the same examples can be invoked with %s.

Where the original Autopus workflow mentions Claude-specific capabilities:

- %s means native Codex elicitation if available, otherwise ask the user directly in one concise message.
- %s maps to Codex subagent spawning.
- Detailed helper references live under %s and %s in this repository.
`,
		"`@auto <subcommand> ...`",
		"`/plugins`",
		"`$auto <subcommand> ...`",
		"`@auto ...`",
		"`$auto ...`",
		"`AskUserQuestion`",
		"`Agent(...)`",
		"`.codex/skills/`",
		"`.codex/rules/autopus/`",
	))

	body = injectAfterFirstHeading(body, aliasNote)
	if frontmatter == "" {
		frontmatter = strings.TrimSpace(fmt.Sprintf(`---
name: auto
description: >
  Autopus Codex router skill. Use when the user wants %s or %s workflows such as plan, go, fix, review, sync, canary, and idea.
---`, "`@auto ...`", "`$auto ...`"))
	}

	return frontmatter + "\n\n" + strings.TrimSpace(body) + "\n", nil
}

func (a *Adapter) renderTemplateAsSkill(cfg *config.HarnessConfig, spec standardSkillSpec) (string, error) {
	tmplContent, err := templates.FS.ReadFile(spec.TemplatePath)
	if err != nil {
		return "", fmt.Errorf("codex skill 템플릿 읽기 실패 %s: %w", spec.TemplatePath, err)
	}

	rendered, err := a.engine.RenderString(string(tmplContent), cfg)
	if err != nil {
		return "", fmt.Errorf("codex skill 템플릿 렌더링 실패 %s: %w", spec.Name, err)
	}

	_, body := splitSkillFrontmatter(rendered)
	if strings.TrimSpace(body) == "" {
		body = rendered
	}

	body = strings.TrimSpace(body)
	body = pkgcontent.ReplacePlatformReferences(body, "codex")
	body = normalizeCodexSkillBody(body, strings.TrimPrefix(spec.Name, "auto-"))
	if !strings.Contains(body, "## Codex Invocation") {
		invocationNote := strings.TrimSpace(fmt.Sprintf(`
## Codex Invocation

You can invoke this workflow through any of these compatible surfaces:

- %s — preferred when the local Autopus plugin is installed
- %s — direct repository skill invocation
- %s — via the router skill

Load and follow any helper documents referenced from this file under %s and %s.
`,
			fmt.Sprintf("`@auto %s ...`", strings.TrimPrefix(spec.Name, "auto-")),
			fmt.Sprintf("`$%s ...`", spec.Name),
			fmt.Sprintf("`$auto %s ...`", strings.TrimPrefix(spec.Name, "auto-")),
			"`.codex/skills/`",
			"`.codex/rules/autopus/`",
		))
		body = injectAfterFirstHeading(body, invocationNote)
	}

	frontmatter := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\n---", spec.Name, spec.Description)
	return frontmatter + "\n\n" + body + "\n", nil
}

func normalizeCodexSkillBody(body, subcommand string) string {
	body = normalizeCodexInvocationBody(body)
	if subcommand == "" {
		return body
	}

	replacer := strings.NewReplacer(
		fmt.Sprintf("@auto-%s", subcommand), fmt.Sprintf("@auto %s", subcommand),
		fmt.Sprintf("$auto-%s", subcommand), fmt.Sprintf("$auto %s", subcommand),
	)
	return replacer.Replace(body)
}

func normalizeCodexInvocationBody(body string) string {
	replacer := strings.NewReplacer(
		"`/auto ", "`@auto ",
		"/auto ", "@auto ",
		"`/auto`", "`@auto`",
	)
	return replacer.Replace(body)
}

func (a *Adapter) renderPluginManifestJSON() (string, error) {
	doc := pluginManifest{
		Name:        "auto",
		Version:     "1.0.0",
		Description: "Autopus workflow router for Codex: plan, go, fix, review, sync, canary, and idea.",
		Author: pluginAuthor{
			Name:  "Autopus",
			Email: "noreply@autopus.co",
			URL:   "https://autopus.co",
		},
		Homepage:   "https://autopus.co",
		Repository: "https://github.com/insajin/autopus-adk",
		License:    "Apache-2.0",
		Keywords:   []string{"autopus", "workflow", "planning", "codex", "multi-provider"},
		Skills:     "./skills",
		Interface: pluginInterface{
			DisplayName:       "Auto",
			ShortDescription:  "Autopus workflow router for Codex",
			LongDescription:   "Run Autopus plan/go/fix/review/sync/canary/idea workflows from Codex with a local plugin plus repository-managed helper docs.",
			DeveloperName:     "Autopus",
			Category:          "Developer Tools",
			Capabilities:      []string{"Interactive", "Write", "Planning"},
			WebsiteURL:        "https://autopus.co",
			PrivacyPolicyURL:  "https://autopus.co/privacy",
			TermsOfServiceURL: "https://autopus.co/terms",
			DefaultPrompt: []string{
				"@auto plan \"새 기능 요구사항을 SPEC으로 정리해줘\"",
				"@auto go SPEC-EXAMPLE-001",
				"@auto idea \"새 워크플로우를 멀티 프로바이더로 토론해줘\" --multi",
			},
			BrandColor: "#0F766E",
		},
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("plugin.json 직렬화 실패: %w", err)
	}

	return string(data) + "\n", nil
}

func (a *Adapter) renderMarketplaceJSON() (string, error) {
	doc := marketplaceDoc{
		Name: "autopus-local",
		Interface: marketplaceUI{
			DisplayName: "Autopus Local",
		},
		Plugins: []marketplaceEntry{
			{
				Name: "auto",
				Source: marketplaceSource{
					Source: "local",
					Path:   "./.autopus/plugins/auto",
				},
				Policy: marketplacePolicy{
					Installation:   "AVAILABLE",
					Authentication: "ON_INSTALL",
				},
				Category: "Developer Tools",
			},
		},
	}

	existingPath := filepath.Join(a.root, ".agents", "plugins", "marketplace.json")
	if data, err := os.ReadFile(existingPath); err == nil {
		var existing marketplaceDoc
		if jsonErr := json.Unmarshal(data, &existing); jsonErr == nil {
			if existing.Name != "" {
				doc.Name = existing.Name
			}
			if existing.Interface.DisplayName != "" {
				doc.Interface.DisplayName = existing.Interface.DisplayName
			}
			updated := false
			for i := range existing.Plugins {
				if existing.Plugins[i].Name == "auto" {
					existing.Plugins[i] = doc.Plugins[0]
					updated = true
					break
				}
			}
			if !updated {
				existing.Plugins = append(existing.Plugins, doc.Plugins[0])
			}
			doc.Plugins = existing.Plugins
		}
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marketplace.json 직렬화 실패: %w", err)
	}

	return string(data) + "\n", nil
}

func newSkillMapping(targetPath, content string) adapter.FileMapping {
	return adapter.FileMapping{
		TargetPath:      targetPath,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(content),
		Content:         []byte(content),
	}
}

func splitSkillFrontmatter(content string) (string, string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", strings.TrimSpace(content)
	}

	rest := strings.TrimPrefix(content, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", strings.TrimSpace(content)
	}

	frontmatter := strings.TrimSpace(content[:len("---\n")+idx+len("\n---")])
	body := strings.TrimSpace(rest[idx+len("\n---\n"):])
	return frontmatter, body
}

func injectAfterFirstHeading(body, block string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			out := make([]string, 0, len(lines)+4)
			out = append(out, lines[:i+1]...)
			out = append(out, "")
			out = append(out, block)
			out = append(out, "")
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}
	return block + "\n\n" + body
}
