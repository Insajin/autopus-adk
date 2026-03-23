// Package gemini는 Gemini CLI 플랫폼 어댑터를 구현한다.
package gemini

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
	adapterName = "gemini-cli"
	cliBinary   = "gemini"
	adapterVer  = "1.0.0"
)

// Adapter는 Gemini CLI 플랫폼 어댑터이다.
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

// SupportsHooks는 false를 반환한다. Gemini CLI는 훅을 지원하지 않는다.
func (a *Adapter) SupportsHooks() bool { return false }

// Detect는 PATH에서 gemini 바이너리 설치 여부를 확인한다.
func (a *Adapter) Detect(_ context.Context) (bool, error) {
	_, err := exec.LookPath(cliBinary)
	return err == nil, nil
}

// Generate는 하네스 설정에 기반하여 Gemini CLI 파일을 생성한다.
func (a *Adapter) Generate(_ context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// .gemini/skills/autopus/ 디렉터리 생성
	geminiSkillDir := filepath.Join(a.root, ".gemini", "skills", "autopus")
	if err := os.MkdirAll(geminiSkillDir, 0755); err != nil {
		return nil, fmt.Errorf(".gemini/skills/autopus 디렉터리 생성 실패: %w", err)
	}

	// .agents/skills/ 크로스플랫폼 앨리어스 디렉터리 생성
	agentsSkillsDir := filepath.Join(a.root, ".agents", "skills")
	if err := os.MkdirAll(agentsSkillsDir, 0755); err != nil {
		return nil, fmt.Errorf(".agents/skills 디렉터리 생성 실패: %w", err)
	}

	var files []adapter.FileMapping

	// GEMINI.md 생성 (마커 섹션 방식)
	geminiMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("GEMINI.md 마커 주입 실패: %w", err)
	}

	geminiMDPath := filepath.Join(a.root, "GEMINI.md")
	if err := os.WriteFile(geminiMDPath, []byte(geminiMD), 0644); err != nil {
		return nil, fmt.Errorf("GEMINI.md 쓰기 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      "GEMINI.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        checksum(geminiMD),
		Content:         []byte(geminiMD),
	})

	// 스킬 템플릿 렌더링 후 .gemini/skills/autopus/{skill}/SKILL.md 에 작성
	skillFiles, err := a.renderSkillTemplates(cfg, geminiSkillDir)
	if err != nil {
		return nil, fmt.Errorf("제미니 스킬 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, skillFiles...)

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(geminiMD),
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

	// GEMINI.md
	geminiMD, err := a.injectMarkerSection(cfg)
	if err != nil {
		return nil, fmt.Errorf("GEMINI.md 마커 주입 실패: %w", err)
	}
	files = append(files, adapter.FileMapping{
		TargetPath:      "GEMINI.md",
		OverwritePolicy: adapter.OverwriteMarker,
		Checksum:        checksum(geminiMD),
		Content:         []byte(geminiMD),
	})

	// 스킬 템플릿
	entries, err := templates.FS.ReadDir("gemini/skills")
	if err != nil {
		return nil, fmt.Errorf("제미니 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name()
		tmplPath := "gemini/skills/" + skillName + "/SKILL.md.tmpl"
		tmplContent, err := templates.FS.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("제미니 스킬 템플릿 읽기 실패 %s: %w", tmplPath, err)
		}
		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("제미니 스킬 템플릿 렌더링 실패 %s: %w", skillName, err)
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".gemini", "skills", "autopus", skillName, "SKILL.md"),
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

	// GEMINI.md 확인
	geminiMDPath := filepath.Join(a.root, "GEMINI.md")
	data, err := os.ReadFile(geminiMDPath)
	if err != nil {
		errs = append(errs, adapter.ValidationError{
			File:    "GEMINI.md",
			Message: "GEMINI.md를 읽을 수 없음",
			Level:   "error",
		})
		return errs, nil
	}
	if !strings.Contains(string(data), markerBegin) {
		errs = append(errs, adapter.ValidationError{
			File:    "GEMINI.md",
			Message: "AUTOPUS 마커 섹션이 없음",
			Level:   "warning",
		})
	}

	// .gemini/skills/autopus/{skill}/SKILL.md 확인 (auto-plan, auto-go 등)
	skillDirs := []string{"auto-plan", "auto-go", "auto-fix", "auto-sync", "auto-review"}
	for _, sd := range skillDirs {
		skillPath := filepath.Join(a.root, ".gemini", "skills", "autopus", sd, "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			errs = append(errs, adapter.ValidationError{
				File:    skillPath,
				Message: fmt.Sprintf("SKILL.md가 없음: %s", sd),
				Level:   "error",
			})
		}
	}

	// .agents/skills 확인
	agentsPath := filepath.Join(a.root, ".agents", "skills")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".agents/skills",
			Message: ".agents/skills 디렉터리가 없음",
			Level:   "warning",
		})
	}

	return errs, nil
}

// Clean은 어댑터가 생성한 파일을 제거한다.
func (a *Adapter) Clean(_ context.Context) error {
	// .gemini/skills 디렉터리 제거
	if err := os.RemoveAll(filepath.Join(a.root, ".gemini", "skills")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".gemini/skills 제거 실패: %w", err)
	}

	// .agents/skills 디렉터리 제거
	if err := os.RemoveAll(filepath.Join(a.root, ".agents", "skills")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".agents/skills 제거 실패: %w", err)
	}

	// GEMINI.md에서 마커 섹션 제거
	geminiPath := filepath.Join(a.root, "GEMINI.md")
	data, err := os.ReadFile(geminiPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("GEMINI.md 읽기 실패: %w", err)
	}
	cleaned := removeMarkerSection(string(data))
	return os.WriteFile(geminiPath, []byte(cleaned), 0644)
}

// InstallHooks는 Gemini CLI에서 no-op이다 (SupportsHooks=false).
func (a *Adapter) InstallHooks(_ context.Context, _ []adapter.HookConfig, _ *adapter.PermissionSet) error {
	return nil
}

// injectMarkerSection은 GEMINI.md의 AUTOPUS 마커 섹션을 생성하거나 업데이트한다.
func (a *Adapter) injectMarkerSection(cfg *config.HarnessConfig) (string, error) {
	geminiMDPath := filepath.Join(a.root, "GEMINI.md")

	var existing string
	if data, err := os.ReadFile(geminiMDPath); err == nil {
		existing = string(data)
	}

	sectionContent, err := a.engine.RenderString(geminiMDTemplate, cfg)
	if err != nil {
		return "", fmt.Errorf("GEMINI.md 템플릿 렌더링 실패: %w", err)
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

// renderSkillTemplates는 embedded FS에서 Gemini 스킬 템플릿을 읽어 렌더링 후
// .gemini/skills/autopus/{skill}/SKILL.md 에 저장한다.
// geminiSkillBaseDir는 .gemini/skills/autopus 의 절대 경로이다.
func (a *Adapter) renderSkillTemplates(cfg *config.HarnessConfig, geminiSkillBaseDir string) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	// gemini/skills 하위의 스킬 디렉터리 목록 조회
	entries, err := templates.FS.ReadDir("gemini/skills")
	if err != nil {
		return nil, fmt.Errorf("제미니 스킬 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillName := entry.Name() // 예: "auto-plan"

		tmplPath := "gemini/skills/" + skillName + "/SKILL.md.tmpl"
		tmplContent, err := templates.FS.ReadFile(tmplPath)
		if err != nil {
			return nil, fmt.Errorf("제미니 스킬 템플릿 읽기 실패 %s: %w", tmplPath, err)
		}

		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("제미니 스킬 템플릿 렌더링 실패 %s: %w", skillName, err)
		}

		// .gemini/skills/autopus/{skill}/ 디렉터리 생성
		skillDir := filepath.Join(geminiSkillBaseDir, skillName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return nil, fmt.Errorf("제미니 스킬 디렉터리 생성 실패 %s: %w", skillDir, err)
		}

		destPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(destPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("제미니 SKILL.md 쓰기 실패 %s: %w", destPath, err)
		}

		relPath := filepath.Join(".gemini", "skills", "autopus", skillName, "SKILL.md")
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// geminiMDTemplate은 GEMINI.md AUTOPUS 섹션 템플릿이다.
const geminiMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}

## 스킬 디렉터리

- Gemini Skills: .gemini/skills/
- Cross-platform: .agents/skills/

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

