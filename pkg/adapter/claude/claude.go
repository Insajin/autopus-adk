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
		filepath.Join(a.root, ".claude", "commands"),
		filepath.Join(a.root, ".claude", "agents", "autopus"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패 %s: %w", d, err)
		}
	}

	// 레거시 .claude/commands/autopus/ 디렉터리 정리 (v1 → v2 마이그레이션)
	// auto update 시 구 개별 커맨드 파일이 남아 /autopus:* 접두사로 노출되는 것을 방지
	legacyCmdDir := filepath.Join(a.root, ".claude", "commands", "autopus")
	if _, err := os.Stat(legacyCmdDir); err == nil {
		if err := os.RemoveAll(legacyCmdDir); err != nil {
			return nil, fmt.Errorf("레거시 커맨드 디렉터리 정리 실패 %s: %w", legacyCmdDir, err)
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

	// 단일 라우터 커맨드 렌더링 후 .claude/commands/auto.md 에 작성
	commandFiles, err := a.renderRouterCommand(cfg)
	if err != nil {
		return nil, fmt.Errorf("커맨드 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, commandFiles...)

	// .mcp.json 생성
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

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(claudeMD),
	}

	// 매니페스트 저장
	m := adapter.ManifestFromFiles(adapterName, pf)
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}

	return pf, nil
}

// Update는 매니페스트 기반으로 파일을 업데이트한다.
// 사용자가 수정한 파일은 백업 후 덮어쓰고, 삭제한 파일은 재생성하지 않는다.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// 이전 매니페스트 로드
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	// 매니페스트가 없으면 init 이전 상태 → Generate로 폴백
	if oldManifest == nil {
		pf, err := a.Generate(ctx, cfg)
		if err != nil {
			return nil, err
		}
		// 매니페스트 저장
		m := adapter.ManifestFromFiles(adapterName, pf)
		if saveErr := m.Save(a.root); saveErr != nil {
			return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
		}
		return pf, nil
	}

	// 새 파일 목록 생성 (디스크에 쓰지 않고 내용만 준비)
	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}

	// 백업 디렉터리 (필요 시 생성)
	var backupDir string
	var results []adapter.UpdateResult

	var finalFiles []adapter.FileMapping
	for _, f := range newFiles {
		action := adapter.ResolveAction(a.root, f.TargetPath, f.OverwritePolicy, oldManifest)

		switch action {
		case adapter.ActionSkip:
			results = append(results, adapter.UpdateResult{
				Path:   f.TargetPath,
				Action: adapter.ActionSkip,
			})
			continue

		case adapter.ActionBackup:
			if backupDir == "" {
				backupDir, err = adapter.CreateBackupDir(a.root)
				if err != nil {
					return nil, err
				}
			}
			backupPath, backupErr := adapter.BackupFile(a.root, f.TargetPath, backupDir)
			if backupErr != nil {
				return nil, backupErr
			}
			results = append(results, adapter.UpdateResult{
				Path:       f.TargetPath,
				Action:     adapter.ActionBackup,
				BackupPath: backupPath,
			})
		}

		// 파일 쓰기 (Overwrite, Backup, Create 모두 쓰기 수행)
		targetPath := filepath.Join(a.root, f.TargetPath)
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패 %s: %w", targetDir, err)
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

	// 새 매니페스트 저장
	m := adapter.ManifestFromFiles(adapterName, pf)
	// 스킵된 파일도 매니페스트에 기록 (삭제 상태 유지)
	for _, r := range results {
		if r.Action == adapter.ActionSkip {
			if prev, ok := oldManifest.Files[r.Path]; ok {
				m.Files[r.Path] = prev
			}
		}
	}
	if saveErr := m.Save(a.root); saveErr != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
	}

	// 백업 알림
	if backupDir != "" {
		fmt.Fprintf(os.Stderr, "  백업됨: %s\n", backupDir)
	}

	return pf, nil
}

// prepareFiles는 Generate와 동일한 파일을 준비하되, 디스크에 쓰지 않고 내용만 반환한다.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

	// CLAUDE.md 마커 섹션
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

	// 라우터 커맨드
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

	// Full 모드: 스킬/에이전트
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

	// 렌더링된 JSON 파싱
	var newMCP map[string]interface{}
	if err := json.Unmarshal([]byte(rendered), &newMCP); err != nil {
		return nil, fmt.Errorf("MCP JSON 파싱 실패: %w", err)
	}

	// 기존 .mcp.json이 있으면 사용자 서버를 보존하며 머지
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

// Validate는 설치된 파일의 유효성을 검증한다.
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

	requiredDirs := []string{
		filepath.Join(".claude", "rules", "autopus"),
		filepath.Join(".claude", "skills", "autopus"),
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

	// 라우터 커맨드 파일 확인
	autoMDPath := filepath.Join(".claude", "commands", "auto.md")
	if _, err := os.Stat(filepath.Join(a.root, autoMDPath)); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    autoMDPath,
			Message: "라우터 커맨드 파일이 없음: .claude/commands/auto.md",
			Level:   "error",
		})
	}

	// .mcp.json 확인
	mcpPath := filepath.Join(a.root, ".mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".mcp.json",
			Message: "MCP 설정 파일이 없음: .mcp.json",
			Level:   "warning",
		})
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
		filepath.Join(a.root, ".claude", "commands", "autopus"), // 구 디렉터리 정리
		filepath.Join(a.root, ".claude", "agents", "autopus"),
	}
	for _, d := range autopusDirs {
		if err := os.RemoveAll(d); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("디렉터리 제거 실패 %s: %w", d, err)
		}
	}

	// 라우터 커맨드 파일 삭제
	autoMDPath := filepath.Join(a.root, ".claude", "commands", "auto.md")
	if err := os.Remove(autoMDPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("라우터 커맨드 삭제 실패: %w", err)
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
- Commands: .claude/commands/auto.md
- Agents: .claude/agents/autopus/
`
