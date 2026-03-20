// Package codex는 Codex 플랫폼 어댑터를 구현한다.
package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
	"github.com/insajin/autopus-adk/templates"
)

const (
	markerBegin = "<!-- AUTOPUS:BEGIN -->"
	markerEnd   = "<!-- AUTOPUS:END -->"
	adapterName = "codex"
	cliBinary   = "codex"
	adapterVer  = "1.0.0"
)

// Adapter는 Codex 플랫폼 어댑터이다.
type Adapter struct {
	root   string
	engine *tmpl.Engine
}

// New는 현재 디렉터리를 루트로 하는 어댑터를 생성한다.
func New() *Adapter {
	return &Adapter{root: ".", engine: tmpl.New()}
}

// NewWithRoot는 지정된 루트 경로로 어댑터를 생성한다.
func NewWithRoot(root string) *Adapter {
	return &Adapter{root: root, engine: tmpl.New()}
}

func (a *Adapter) Name() string      { return adapterName }
func (a *Adapter) Version() string   { return adapterVer }
func (a *Adapter) CLIBinary() string { return cliBinary }

// SupportsHooks는 false를 반환한다. Codex는 Git 훅 폴백을 사용한다.
func (a *Adapter) SupportsHooks() bool { return false }

// Detect는 PATH에서 codex 바이너리 설치 여부를 확인한다.
func (a *Adapter) Detect(_ context.Context) (bool, error) {
	_, err := exec.LookPath(cliBinary)
	return err == nil, nil
}

// Generate는 하네스 설정에 기반하여 Codex 파일을 생성한다.
func (a *Adapter) Generate(_ context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// .codex/skills/ 디렉터리 생성
	skillsDir := filepath.Join(a.root, ".codex", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf(".codex/skills 디렉터리 생성 실패: %w", err)
	}

	// AGENTS.md 생성 (마커 섹션 방식)
	agentsMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("AGENTS.md 마커 주입 실패: %w", err)
	}

	agentsPath := filepath.Join(a.root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(agentsMD), 0644); err != nil {
		return nil, fmt.Errorf("AGENTS.md 쓰기 실패: %w", err)
	}

	files := []adapter.FileMapping{
		{
			TargetPath:      "AGENTS.md",
			OverwritePolicy: adapter.OverwriteMarker,
			Checksum:        checksum(agentsMD),
			Content:         []byte(agentsMD),
		},
	}

	// 스킬 템플릿 렌더링 후 .codex/skills/ 에 작성
	skillFiles, err := a.renderSkillTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("스킬 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, skillFiles...)

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(agentsMD),
	}

	// 매니페스트 저장
	m := adapter.ManifestFromFiles(adapterName, pf)
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}

	return pf, nil
}

// Update는 매니페스트 기반으로 파일을 업데이트한다.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	if oldManifest == nil {
		return a.Generate(ctx, cfg)
	}

	// 새 파일 준비
	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}

	var backupDir string
	var finalFiles []adapter.FileMapping

	for _, f := range newFiles {
		action := adapter.ResolveAction(a.root, f.TargetPath, f.OverwritePolicy, oldManifest)

		if action == adapter.ActionSkip {
			continue
		}
		if action == adapter.ActionBackup {
			if backupDir == "" {
				backupDir, err = adapter.CreateBackupDir(a.root)
				if err != nil {
					return nil, err
				}
			}
			if _, backupErr := adapter.BackupFile(a.root, f.TargetPath, backupDir); backupErr != nil {
				return nil, backupErr
			}
		}

		targetPath := filepath.Join(a.root, f.TargetPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(targetPath, f.Content, 0644); err != nil {
			return nil, fmt.Errorf("파일 쓰기 실패 %s: %w", f.TargetPath, err)
		}
		finalFiles = append(finalFiles, f)
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	m := adapter.ManifestFromFiles(adapterName, pf)
	if saveErr := m.Save(a.root); saveErr != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
	}

	if backupDir != "" {
		fmt.Fprintf(os.Stderr, "  백업됨: %s\n", backupDir)
	}

	return pf, nil
}

// prepareFiles는 Generate와 동일한 파일을 준비하되 디스크에 쓰지 않는다.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	// AGENTS.md
	agentsMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("AGENTS.md 마커 주입 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      "AGENTS.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        checksum(agentsMD),
		Content:         []byte(agentsMD),
	})

	// 스킬 템플릿
	entries, err := templates.FS.ReadDir("codex/skills")
	if err != nil {
		return nil, fmt.Errorf("코덱스 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}
		skillFile := strings.TrimSuffix(entry.Name(), ".tmpl")
		tmplContent, err := templates.FS.ReadFile("codex/skills/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 읽기 실패 %s: %w", entry.Name(), err)
		}
		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 렌더링 실패 %s: %w", entry.Name(), err)
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "skills", skillFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// Validate는 설치된 파일의 유효성을 검증한다.
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

	// AGENTS.md 존재 확인
	agentsPath := filepath.Join(a.root, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		errs = append(errs, adapter.ValidationError{
			File:    "AGENTS.md",
			Message: "AGENTS.md를 읽을 수 없음",
			Level:   "error",
		})
		return errs, nil
	}

	content := string(data)
	if !strings.Contains(content, markerBegin) || !strings.Contains(content, markerEnd) {
		errs = append(errs, adapter.ValidationError{
			File:    "AGENTS.md",
			Message: "AUTOPUS 마커 섹션이 없음",
			Level:   "warning",
		})
	}

	// .codex/skills 디렉터리 확인
	skillsDir := filepath.Join(a.root, ".codex", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".codex/skills",
			Message: ".codex/skills 디렉터리가 없음",
			Level:   "error",
		})
	}

	return errs, nil
}

// Clean은 어댑터가 생성한 파일을 제거한다.
func (a *Adapter) Clean(_ context.Context) error {
	// .codex/skills 디렉터리 제거
	if err := os.RemoveAll(filepath.Join(a.root, ".codex", "skills")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".codex/skills 제거 실패: %w", err)
	}

	// AGENTS.md에서 마커 섹션 제거
	agentsPath := filepath.Join(a.root, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("AGENTS.md 읽기 실패: %w", err)
	}
	cleaned := removeMarkerSection(string(data))
	return os.WriteFile(agentsPath, []byte(cleaned), 0644)
}

// InstallHooks는 Codex에서 no-op이다 (SupportsHooks=false).
func (a *Adapter) InstallHooks(_ context.Context, _ []adapter.HookConfig) error {
	return nil
}

// injectMarkerSection은 AGENTS.md의 AUTOPUS 마커 섹션을 생성하거나 업데이트한다.
func (a *Adapter) injectMarkerSection(cfg *config.HarnessConfig) (string, error) {
	agentsPath := filepath.Join(a.root, "AGENTS.md")

	var existing string
	if data, err := os.ReadFile(agentsPath); err == nil {
		existing = string(data)
	}

	sectionContent, err := a.engine.RenderString(agentsMDTemplate, cfg)
	if err != nil {
		return "", fmt.Errorf("AGENTS.md 템플릿 렌더링 실패: %w", err)
	}

	newSection := markerBegin + "\n" + sectionContent + "\n" + markerEnd

	if strings.Contains(existing, markerBegin) && strings.Contains(existing, markerEnd) {
		return replaceMarkerSection(existing, newSection), nil
	}

	if existing == "" {
		return newSection + "\n", nil
	}
	return existing + "\n\n" + newSection + "\n", nil
}

var markerRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(markerBegin) + `.*?` + regexp.QuoteMeta(markerEnd))

func replaceMarkerSection(content, newSection string) string {
	return markerRe.ReplaceAllString(content, newSection)
}

func removeMarkerSection(content string) string {
	return strings.TrimSpace(markerRe.ReplaceAllString(content, "")) + "\n"
}

func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// renderSkillTemplates는 embedded FS에서 Codex 스킬 템플릿을 읽어 렌더링 후 .codex/skills/ 에 저장한다.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	entries, err := templates.FS.ReadDir("codex/skills")
	if err != nil {
		return nil, fmt.Errorf("코덱스 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".tmpl") {
			continue
		}

		// 스킬명 추출 (예: auto-plan.md.tmpl -> auto-plan.md)
		skillFile := strings.TrimSuffix(name, ".tmpl")

		// 템플릿 내용 읽기
		tmplContent, err := templates.FS.ReadFile("codex/skills/" + name)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 읽기 실패 %s: %w", name, err)
		}

		// 템플릿 렌더링
		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("코덱스 스킬 템플릿 렌더링 실패 %s: %w", name, err)
		}

		// 대상 경로: .codex/skills/{skill}.md
		targetPath := filepath.Join(a.root, ".codex", "skills", skillFile)
		if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("코덱스 스킬 파일 쓰기 실패 %s: %w", targetPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".codex", "skills", skillFile),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// agentsMDTemplate은 AGENTS.md AUTOPUS 섹션 템플릿이다.
const agentsMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}

## 스킬 디렉터리

- Skills: .codex/skills/

## Core Guidelines

### Subagent Delegation

IMPORTANT: Use subagents for complex tasks that modify 3+ files, span multiple domains, or exceed 200 lines of new code. Define clear scope, provide full context, review output before integrating.

### File Size Limit

IMPORTANT: No source code file may exceed 300 lines. Target under 200 lines. Split by type, concern, or layer when approaching the limit. Excluded: generated files (*_generated.go, *.pb.go), documentation (*.md), and config files (*.yaml, *.json).

### Code Review

During review, verify:
- No file exceeds 300 lines (REQUIRED)
- Complex changes use subagent delegation (SUGGESTED)
`
