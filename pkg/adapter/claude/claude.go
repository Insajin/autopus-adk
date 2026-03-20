// Package claude는 Claude Code 플랫폼 어댑터를 구현한다.
package claude

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
	"github.com/insajin/autopus-adk/templates"
)

const (
	markerBegin = "<!-- AUTOPUS:BEGIN -->"
	markerEnd   = "<!-- AUTOPUS:END -->"
	adapterName = "claude-code"
	cliBinary   = "claude"
	adapterVer  = "1.0.0"
)

// Adapter는 Claude Code 플랫폼 어댑터이다.
// @AX:ANCHOR: PlatformAdapter 인터페이스의 claude-code 구현체 — 다수의 CLI 커맨드에서 사용됨
// @AX:REASON: [AUTO] init/update/doctor/platform 커맨드에서 참조
type Adapter struct {
	root   string        // 프로젝트 루트 경로
	engine *tmpl.Engine  // 템플릿 렌더링 엔진
}

// New는 현재 디렉터리를 루트로 하는 어댑터를 생성한다.
func New() *Adapter {
	return &Adapter{
		root:   ".",
		engine: tmpl.New(),
	}
}

// NewWithRoot는 지정된 루트 경로로 어댑터를 생성한다.
func NewWithRoot(root string) *Adapter {
	return &Adapter{
		root:   root,
		engine: tmpl.New(),
	}
}

func (a *Adapter) Name() string    { return adapterName }
func (a *Adapter) Version() string { return adapterVer }
func (a *Adapter) CLIBinary() string { return cliBinary }
func (a *Adapter) SupportsHooks() bool { return true }

// Detect는 PATH에서 claude 바이너리 설치 여부를 확인한다.
func (a *Adapter) Detect(_ context.Context) (bool, error) {
	_, err := exec.LookPath(cliBinary)
	return err == nil, nil
}

// Generate는 하네스 설정에 기반하여 Claude Code 파일을 생성한다.
func (a *Adapter) Generate(_ context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// 필수 디렉터리 생성
	dirs := []string{
		filepath.Join(a.root, ".claude", "rules", "autopus"),
		filepath.Join(a.root, ".claude", "skills", "autopus"),
		filepath.Join(a.root, ".claude", "commands", "autopus"),
		filepath.Join(a.root, ".claude", "agents", "autopus"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패 %s: %w", d, err)
		}
	}

	var files []adapter.FileMapping

	// CLAUDE.md 마커 섹션 처리
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

	// 커맨드 템플릿 렌더링 후 .claude/commands/autopus/ 에 작성
	commandFiles, err := a.renderCommandTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("커맨드 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, commandFiles...)

	// Full 모드: 스킬/에이전트 컨텐츠 파일 복사
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

	return &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(claudeMD),
	}, nil
}

// Update는 기존 파일을 업데이트한다. 체크섬 비교 후 변경된 파일만 업데이트한다.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// Generate와 동일 로직: 마커 섹션만 업데이트하고 사용자 수정 보존
	return a.Generate(ctx, cfg)
}

// Validate는 설치된 파일의 유효성을 검증한다.
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

	requiredDirs := []string{
		filepath.Join(".claude", "rules", "autopus"),
		filepath.Join(".claude", "skills", "autopus"),
		filepath.Join(".claude", "commands", "autopus"),
		filepath.Join(".claude", "agents", "autopus"),
	}
	for _, d := range requiredDirs {
		fullPath := filepath.Join(a.root, d)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			errs = append(errs, adapter.ValidationError{
				File:    d,
				Message: fmt.Sprintf("필수 디렉터리가 없음: %s", d),
				Level:   "error",
			})
		}
	}

	// CLAUDE.md 마커 확인
	claudePath := filepath.Join(a.root, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		errs = append(errs, adapter.ValidationError{
			File:    "CLAUDE.md",
			Message: "CLAUDE.md를 읽을 수 없음",
			Level:   "error",
		})
	} else {
		content := string(data)
		if !strings.Contains(content, markerBegin) || !strings.Contains(content, markerEnd) {
			errs = append(errs, adapter.ValidationError{
				File:    "CLAUDE.md",
				Message: "AUTOPUS 마커 섹션이 없음",
				Level:   "warning",
			})
		}
	}

	return errs, nil
}

// Clean은 어댑터가 생성한 autopus 전용 파일과 디렉터리를 제거한다.
func (a *Adapter) Clean(_ context.Context) error {
	autopusDirs := []string{
		filepath.Join(a.root, ".claude", "rules", "autopus"),
		filepath.Join(a.root, ".claude", "skills", "autopus"),
		filepath.Join(a.root, ".claude", "commands", "autopus"),
		filepath.Join(a.root, ".claude", "agents", "autopus"),
	}
	for _, d := range autopusDirs {
		if err := os.RemoveAll(d); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("디렉터리 제거 실패 %s: %w", d, err)
		}
	}

	// CLAUDE.md에서 마커 섹션 제거
	claudePath := filepath.Join(a.root, "CLAUDE.md")
	data, err := os.ReadFile(claudePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("CLAUDE.md 읽기 실패: %w", err)
	}
	cleaned := removeMarkerSection(string(data))
	return os.WriteFile(claudePath, []byte(cleaned), 0644)
}

// InstallHooks는 .claude/settings.json에 훅 항목을 생성한다.
func (a *Adapter) InstallHooks(_ context.Context, hooks []adapter.HookConfig) error {
	settingsDir := filepath.Join(a.root, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("설정 디렉터리 생성 실패: %w", err)
	}

	settingsPath := filepath.Join(settingsDir, "settings.json")

	// 기존 settings.json 로드 또는 기본 구조 생성
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			settings = make(map[string]interface{})
		}
	} else {
		settings = make(map[string]interface{})
	}

	// 훅 항목 추가
	if len(hooks) > 0 {
		hooksData := make([]map[string]interface{}, 0, len(hooks))
		for _, h := range hooks {
			hooksData = append(hooksData, map[string]interface{}{
				"event":   h.Event,
				"command": h.Command,
				"timeout": h.Timeout,
			})
		}
		settings["hooks"] = hooksData
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("settings.json 직렬화 실패: %w", err)
	}
	return os.WriteFile(settingsPath, out, 0644)
}

// injectMarkerSection은 CLAUDE.md의 AUTOPUS 마커 섹션을 생성하거나 업데이트한다.
func (a *Adapter) injectMarkerSection(cfg *config.HarnessConfig) (string, error) {
	claudePath := filepath.Join(a.root, "CLAUDE.md")

	// 기존 파일 읽기 (없으면 빈 문자열)
	var existing string
	if data, err := os.ReadFile(claudePath); err == nil {
		existing = string(data)
	}

	// 마커 섹션 컨텐츠 생성
	sectionContent, err := a.engine.RenderString(claudeMDTemplate, cfg)
	if err != nil {
		return "", fmt.Errorf("CLAUDE.md 템플릿 렌더링 실패: %w", err)
	}

	newSection := markerBegin + "\n" + sectionContent + "\n" + markerEnd

	// 기존 마커 섹션 교체 또는 추가
	if strings.Contains(existing, markerBegin) && strings.Contains(existing, markerEnd) {
		return replaceMarkerSection(existing, newSection), nil
	}

	// 마커 섹션이 없으면 파일 끝에 추가
	if existing == "" {
		return newSection + "\n", nil
	}
	return existing + "\n\n" + newSection + "\n", nil
}

var markerRe = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(markerBegin) + `.*?` + regexp.QuoteMeta(markerEnd))

// replaceMarkerSection은 기존 마커 섹션을 새 섹션으로 교체한다.
func replaceMarkerSection(content, newSection string) string {
	return markerRe.ReplaceAllString(content, newSection)
}

// removeMarkerSection은 마커 섹션을 완전히 제거한다.
func removeMarkerSection(content string) string {
	return strings.TrimSpace(markerRe.ReplaceAllString(content, "")) + "\n"
}

// checksum은 문자열의 SHA256 체크섬을 반환한다.
func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// fullOnlyCommands는 Full 모드에서만 생성되는 커맨드 목록이다.
var fullOnlyCommands = map[string]bool{
	"go":     true,
	"review": true,
	"secure": true,
}

// renderCommandTemplates는 embedded FS에서 커맨드 템플릿을 읽어 렌더링 후 파일로 저장한다.
// Lite 모드에서는 fullOnlyCommands를 건너뛴다.
func (a *Adapter) renderCommandTemplates(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	// embedded FS에서 claude/commands/*.tmpl 목록 조회
	entries, err := templates.FS.ReadDir("claude/commands")
	if err != nil {
		return nil, fmt.Errorf("커맨드 템플릿 디렉터리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".tmpl") {
			continue
		}

		// 커맨드명 추출 (예: plan.md.tmpl -> plan)
		cmdName := strings.TrimSuffix(strings.TrimSuffix(name, ".tmpl"), ".md")

		// Lite 모드에서 Full 전용 커맨드 건너뜀
		if !cfg.IsFullMode() && fullOnlyCommands[cmdName] {
			continue
		}

		// 템플릿 내용 읽기
		tmplContent, err := templates.FS.ReadFile("claude/commands/" + name)
		if err != nil {
			return nil, fmt.Errorf("커맨드 템플릿 읽기 실패 %s: %w", name, err)
		}

		// 템플릿 렌더링
		rendered, err := a.engine.RenderString(string(tmplContent), cfg)
		if err != nil {
			return nil, fmt.Errorf("커맨드 템플릿 렌더링 실패 %s: %w", name, err)
		}

		// 대상 경로: .claude/commands/autopus/{cmd}.md
		targetName := cmdName + ".md"
		targetPath := filepath.Join(a.root, ".claude", "commands", "autopus", targetName)
		if err := os.WriteFile(targetPath, []byte(rendered), 0644); err != nil {
			return nil, fmt.Errorf("커맨드 파일 쓰기 실패 %s: %w", targetPath, err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".claude", "commands", "autopus", targetName),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(rendered),
			Content:         []byte(rendered),
		})
	}

	return files, nil
}

// copyContentFiles는 embedded content FS에서 파일을 읽어 대상 디렉터리에 복사한다.
// subDir: "skills" 또는 "agents"
// targetRelDir: 대상 상대 경로 (예: ".claude/skills/autopus")
func (a *Adapter) copyContentFiles(cfg *config.HarnessConfig, subDir string, targetRelDir string) ([]adapter.FileMapping, error) {
	_ = cfg // 향후 확장을 위해 보존

	var files []adapter.FileMapping

	entries, err := contentfs.FS.ReadDir(subDir)
	if err != nil {
		return nil, fmt.Errorf("컨텐츠 디렉터리 읽기 실패 %s: %w", subDir, err)
	}

	// 대상 디렉터리 생성
	absTargetDir := filepath.Join(a.root, targetRelDir)
	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		return nil, fmt.Errorf("대상 디렉터리 생성 실패 %s: %w", absTargetDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
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

// claudeMDTemplate은 CLAUDE.md AUTOPUS 섹션 템플릿이다.
const claudeMDTemplate = `# Autopus-ADK Harness

> 이 섹션은 Autopus-ADK에 의해 자동 생성됩니다. 수동으로 편집하지 마세요.

- **프로젝트**: {{.ProjectName}}
- **모드**: {{.Mode}}
- **플랫폼**: {{join ", " .Platforms}}

## 설치된 구성 요소

- Rules: .claude/rules/autopus/
- Skills: .claude/skills/autopus/
- Commands: .claude/commands/autopus/
- Agents: .claude/agents/autopus/
`
