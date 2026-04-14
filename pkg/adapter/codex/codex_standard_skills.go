package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
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
		TemplatePath: "codex/skills/auto-canary.md.tmpl",
	},
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

func newSkillMapping(targetPath, content string) adapter.FileMapping {
	return adapter.FileMapping{
		TargetPath:      targetPath,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        checksum(content),
		Content:         []byte(content),
	}
}
