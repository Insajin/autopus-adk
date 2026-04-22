// Package codex implements the Codex platform adapter.
package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

const (
	adapterName = "codex"
	cliBinary   = "codex"
	adapterVer  = "1.0.0"
)

// Adapter is the Codex platform adapter.
type Adapter struct {
	root   string
	engine *tmpl.Engine
}

// New creates an adapter rooted at the current directory.
func New() *Adapter {
	return &Adapter{root: ".", engine: tmpl.New()}
}

// NewWithRoot creates an adapter rooted at the specified path.
func NewWithRoot(root string) *Adapter {
	return &Adapter{root: root, engine: tmpl.New()}
}

func (a *Adapter) Name() string      { return adapterName }
func (a *Adapter) Version() string   { return adapterVer }
func (a *Adapter) CLIBinary() string { return cliBinary }

// SupportsHooks returns true. Codex supports hooks via .codex/hooks.json.
func (a *Adapter) SupportsHooks() bool { return true }

// Detect checks whether the codex binary is installed in PATH.
func (a *Adapter) Detect(_ context.Context) (bool, error) {
	_, err := exec.LookPath(cliBinary)
	return err == nil, nil
}

// Generate creates Codex platform files based on harness config.
func (a *Adapter) Generate(_ context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	skillsDir := filepath.Join(a.root, ".codex", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf(".codex/skills 디렉터리 생성 실패: %w", err)
	}
	agentSkillsDir := filepath.Join(a.root, ".agents", "skills")
	if err := os.MkdirAll(agentSkillsDir, 0755); err != nil {
		return nil, fmt.Errorf(".agents/skills 디렉터리 생성 실패: %w", err)
	}
	pluginDir := filepath.Join(a.root, ".autopus", "plugins", "auto")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, fmt.Errorf(".autopus/plugins/auto 디렉터리 생성 실패: %w", err)
	}
	marketplaceDir := filepath.Join(a.root, ".agents", "plugins")
	if err := os.MkdirAll(marketplaceDir, 0755); err != nil {
		return nil, fmt.Errorf(".agents/plugins 디렉터리 생성 실패: %w", err)
	}

	files := make([]adapter.FileMapping, 0)
	if codexOwnsSharedSurface(cfg) {
		agentsMD, err := a.injectMarkerSection(cfg)
		if err != nil {
			return nil, fmt.Errorf("AGENTS.md 마커 주입 실패: %w", err)
		}

		agentsPath := filepath.Join(a.root, "AGENTS.md")
		if err := os.WriteFile(agentsPath, []byte(agentsMD), 0644); err != nil {
			return nil, fmt.Errorf("AGENTS.md 쓰기 실패: %w", err)
		}

		files = append(files, adapter.FileMapping{
			TargetPath:      "AGENTS.md",
			OverwritePolicy: adapter.OverwriteMarker,
			Checksum:        checksum(agentsMD),
			Content:         []byte(agentsMD),
		})
	}

	skillFiles, err := a.renderSkillTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("스킬 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, skillFiles...)

	if codexOwnsSharedSurface(cfg) {
		standardSkillFiles, err := a.renderStandardSkills(cfg)
		if err != nil {
			return nil, fmt.Errorf("표준 codex skill 생성 실패: %w", err)
		}
		files = append(files, standardSkillFiles...)
	}

	promptFiles, err := a.renderPromptTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("프롬프트 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, promptFiles...)

	pluginFiles, err := a.renderPluginFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex plugin 생성 실패: %w", err)
	}
	files = append(files, pluginFiles...)

	// Agents (TOML files)
	agentFiles, err := a.generateAgents(cfg)
	if err != nil {
		return nil, fmt.Errorf("agent 생성 실패: %w", err)
	}
	files = append(files, agentFiles...)

	// Rules (separate files)
	ruleFiles, err := a.generateRuleFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("rule 파일 생성 실패: %w", err)
	}
	files = append(files, ruleFiles...)

	// Hooks (hooks.json)
	hookFiles, err := a.generateHooks(cfg)
	if err != nil {
		return nil, fmt.Errorf("hooks 생성 실패: %w", err)
	}
	files = append(files, hookFiles...)

	// Config (config.toml)
	configFiles, err := a.generateConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("config 생성 실패: %w", err)
	}
	files = append(files, configFiles...)

	// Git hooks fallback
	if err := a.installGitHooks(cfg); err != nil {
		return nil, fmt.Errorf("git hooks 설치 실패: %w", err)
	}
	gitHookFiles, err := a.prepareGitHookFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("git hooks 준비 실패: %w", err)
	}
	files = append(files, gitHookFiles...)

	if err := a.cleanupDeprecatedSurface(); err != nil {
		return nil, err
	}

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(fmt.Sprintf("%d", len(files))),
	}

	m := adapter.ManifestFromFiles(adapterName, filterCodexManifestFiles(cfg, pf))
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}

	return pf, nil
}

// Update updates files based on manifest diff.
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
	if err := a.cleanupStaleManagedSurfaces(oldManifest, newFiles, &backupDir); err != nil {
		return nil, err
	}

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

	if err := a.cleanupDeprecatedSurface(); err != nil {
		return nil, err
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	m := adapter.ManifestFromFiles(adapterName, filterCodexManifestFiles(cfg, pf))
	if saveErr := m.Save(a.root); saveErr != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", saveErr)
	}

	if backupDir != "" {
		fmt.Fprintf(os.Stderr, "  백업됨: %s\n", backupDir)
	}

	return pf, nil
}

func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func codexOwnsSharedSurface(cfg *config.HarnessConfig) bool {
	return cfg == nil || !containsPlatform(cfg.Platforms, "opencode")
}

func filterCodexManifestFiles(cfg *config.HarnessConfig, pf *adapter.PlatformFiles) *adapter.PlatformFiles {
	if pf == nil || cfg == nil {
		return pf
	}
	if !containsPlatform(cfg.Platforms, "opencode") {
		return pf
	}

	filtered := make([]adapter.FileMapping, 0, len(pf.Files))
	for _, f := range pf.Files {
		if f.TargetPath == "AGENTS.md" {
			continue
		}
		if strings.HasPrefix(f.TargetPath, filepath.Join(".agents", "skills")+string(os.PathSeparator)) {
			continue
		}
		filtered = append(filtered, f)
	}

	return &adapter.PlatformFiles{
		Files:    filtered,
		Checksum: pf.Checksum,
	}
}
