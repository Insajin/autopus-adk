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
	"github.com/insajin/autopus-adk/templates"
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

	skillFiles, err := a.renderSkillTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("스킬 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, skillFiles...)

	promptFiles, err := a.renderPromptTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("프롬프트 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, promptFiles...)

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

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(agentsMD),
	}

	m := adapter.ManifestFromFiles(adapterName, pf)
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

// prepareFiles prepares files without writing to disk.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

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

	// Extended skills from content/skills/ via transformer
	extSkillFiles, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill 준비 실패: %w", err)
	}
	files = append(files, extSkillFiles...)

	promptFiles, err := a.preparePromptFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex prompt 템플릿 준비 실패: %w", err)
	}
	files = append(files, promptFiles...)

	// Rules (separate files)
	rulePrepFiles, err := a.prepareRuleMappings(cfg)
	if err != nil {
		return nil, fmt.Errorf("codex rule 준비 실패: %w", err)
	}
	files = append(files, rulePrepFiles...)

	// Agents (TOML files)
	agentPrepFiles, err := a.prepareAgentFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("agent 준비 실패: %w", err)
	}
	files = append(files, agentPrepFiles...)

	// Hooks (hooks.json)
	hooksPrepFiles, err := a.prepareHooksFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("hooks 준비 실패: %w", err)
	}
	files = append(files, hooksPrepFiles...)

	// Config (config.toml)
	configPrepFiles, err := a.prepareConfigFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("config 준비 실패: %w", err)
	}
	files = append(files, configPrepFiles...)

	return files, nil
}

func checksum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
