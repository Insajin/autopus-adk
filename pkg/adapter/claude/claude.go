// Package claude는 Claude Code 플랫폼 어댑터를 구현한다.
package claude

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
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
	root   string       // project root path
	engine *tmpl.Engine // template rendering engine
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

func (a *Adapter) Name() string       { return adapterName }
func (a *Adapter) Version() string    { return adapterVer }
func (a *Adapter) CLIBinary() string  { return cliBinary }
func (a *Adapter) SupportsHooks() bool { return true }

// Detect는 PATH에서 claude 바이너리 설치 여부를 확인한다.
func (a *Adapter) Detect(_ context.Context) (bool, error) {
	_, err := exec.LookPath(cliBinary)
	return err == nil, nil
}

// Generate는 하네스 설정에 기반하여 Claude Code 파일을 생성한다.
func (a *Adapter) Generate(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// Create required directories
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

	// Clean up legacy .claude/commands/autopus/ directory (v1 → v2 migration)
	legacyCmdDir := filepath.Join(a.root, ".claude", "commands", "autopus")
	if _, err := os.Stat(legacyCmdDir); err == nil {
		if err := os.RemoveAll(legacyCmdDir); err != nil {
			return nil, fmt.Errorf("레거시 커맨드 디렉터리 정리 실패 %s: %w", legacyCmdDir, err)
		}
	}

	var files []adapter.FileMapping

	// CLAUDE.md marker section
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

	// Render router command → .claude/commands/auto.md
	commandFiles, err := a.renderRouterCommand(cfg)
	if err != nil {
		return nil, fmt.Errorf("커맨드 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, commandFiles...)

	// Generate .mcp.json
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

	// Install hooks and permissions to .claude/settings.json
	if err := a.applyHooksAndPermissions(ctx, cfg); err != nil {
		return nil, err
	}

	// Copy rules content files (all modes)
	ruleFiles, err := a.copyContentFiles(cfg, "rules", filepath.Join(".claude", "rules", "autopus"))
	if err != nil {
		return nil, fmt.Errorf("룰 파일 복사 실패: %w", err)
	}
	files = append(files, ruleFiles...)

	// Render and write file-size-limit.md from template (stack/framework-aware)
	fileSizeRule, err := a.prepareFileSizeLimitRule(cfg)
	if err != nil {
		return nil, fmt.Errorf("file-size-limit 룰 생성 실패: %w", err)
	}
	destPath := filepath.Join(a.root, fileSizeRule.TargetPath)
	if err := os.WriteFile(destPath, fileSizeRule.Content, 0644); err != nil {
		return nil, fmt.Errorf("file-size-limit.md 쓰기 실패: %w", err)
	}
	files = append(files, fileSizeRule)

	// Copy statusline script
	statusFiles, err := a.copyStatusline()
	if err != nil {
		return nil, fmt.Errorf("statusline 복사 실패: %w", err)
	}
	files = append(files, statusFiles...)

	// Copy hooks to .claude/hooks/autopus/
	hookFiles, err := a.copyContentFiles(cfg, "hooks", filepath.Join(".claude", "hooks", "autopus"))
	if err != nil {
		return nil, fmt.Errorf("훅 파일 복사 실패: %w", err)
	}
	files = append(files, hookFiles...)

	// Full mode: copy skills/agents content files
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

	// Save manifest
	m := adapter.ManifestFromFiles(adapterName, pf)
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}

	return pf, nil
}

// Update는 매니페스트 기반으로 파일을 업데이트한다.
// 사용자가 수정한 파일은 백업 후 덮어쓰고, 삭제한 파일은 재생성하지 않는다.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	// Load previous manifest
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	// No manifest → Generate as fallback
	if oldManifest == nil {
		pf, err := a.Generate(ctx, cfg)
		if err != nil {
			return nil, err
		}
		m := adapter.ManifestFromFiles(adapterName, pf)
		if saveErr := m.Save(a.root); saveErr != nil {
			return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
		}
		return pf, nil
	}

	// Prepare new file list without writing to disk
	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}

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

		// Write file (Overwrite, Backup, Create all write)
		targetPath := filepath.Join(a.root, f.TargetPath)
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, fmt.Errorf("디렉터리 생성 실패 %s: %w", targetDir, err)
		}
		perm := os.FileMode(0644)
		if filepath.Ext(f.TargetPath) == ".sh" {
			perm = 0755
		}
		if err := os.WriteFile(targetPath, f.Content, perm); err != nil {
			return nil, fmt.Errorf("파일 쓰기 실패 %s: %w", f.TargetPath, err)
		}
		finalFiles = append(finalFiles, f)
	}

	// Install hooks and permissions to .claude/settings.json
	if err := a.applyHooksAndPermissions(ctx, cfg); err != nil {
		return nil, err
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	// Save new manifest (include skipped files from old manifest)
	m := adapter.ManifestFromFiles(adapterName, pf)
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

	if backupDir != "" {
		fmt.Fprintf(os.Stderr, "  백업됨: %s\n", backupDir)
	}

	return pf, nil
}

// checksum은 문자열의 SHA256 체크섬을 반환한다.
func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
