// Package gemini implements the Gemini CLI platform adapter.
package gemini

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

const (
	adapterName = "gemini-cli"
	cliBinary   = "gemini"
	adapterVer  = "1.0.0"
)

// Adapter is the Gemini CLI platform adapter.
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

// SupportsHooks returns true. Gemini CLI supports hooks via .gemini/settings.json.
func (a *Adapter) SupportsHooks() bool { return true }

// Detect checks whether the gemini binary is installed on PATH.
func (a *Adapter) Detect(_ context.Context) (bool, error) {
	_, err := exec.LookPath(cliBinary)
	return err == nil, nil
}

// Generate creates Gemini CLI files based on the harness config.
func (a *Adapter) Generate(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	geminiSkillDir := filepath.Join(a.root, ".gemini", "skills", "autopus")
	if err := os.MkdirAll(geminiSkillDir, 0755); err != nil {
		return nil, fmt.Errorf(".gemini/skills/autopus 디렉터리 생성 실패: %w", err)
	}

	agentsSkillsDir := filepath.Join(a.root, ".agents", "skills")
	if err := os.MkdirAll(agentsSkillsDir, 0755); err != nil {
		return nil, fmt.Errorf(".agents/skills 디렉터리 생성 실패: %w", err)
	}

	var files []adapter.FileMapping

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

	skillFiles, err := a.renderSkillTemplates(cfg, geminiSkillDir)
	if err != nil {
		return nil, fmt.Errorf("제미니 스킬 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, skillFiles...)

	cmdFiles, err := a.renderCommandTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("제미니 커맨드 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, cmdFiles...)

	ruleFiles, err := a.renderRuleTemplates(cfg)
	if err != nil {
		return nil, fmt.Errorf("제미니 룰 템플릿 렌더링 실패: %w", err)
	}
	files = append(files, ruleFiles...)

	// Copy agent content files (full mode)
	if cfg.IsFullMode() {
		agentFiles, err := a.renderAgentFiles()
		if err != nil {
			return nil, fmt.Errorf("제미니 에이전트 파일 복사 실패: %w", err)
		}
		files = append(files, agentFiles...)
	}

	// Generate settings.json (MCP servers, base config)
	settingsFiles, err := a.generateSettings(cfg)
	if err != nil {
		return nil, fmt.Errorf("제미니 설정 생성 실패: %w", err)
	}
	for _, sf := range settingsFiles {
		destPath := filepath.Join(a.root, sf.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("설정 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(destPath, sf.Content, 0644); err != nil {
			return nil, fmt.Errorf("설정 파일 쓰기 실패: %w", err)
		}
	}
	files = append(files, settingsFiles...)

	// Install hooks and permissions to .gemini/settings.json
	if err := a.applyHooksAndPermissions(ctx, cfg); err != nil {
		return nil, fmt.Errorf("제미니 훅/권한 설치 실패: %w", err)
	}

	pf := &adapter.PlatformFiles{
		Files:    files,
		Checksum: checksum(geminiMD),
	}

	m := adapter.ManifestFromFiles(adapterName, pf)
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}

	return pf, nil
}

// Update updates files based on the manifest.
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

// prepareFiles prepares the same files as Generate but without writing to disk.
func (a *Adapter) prepareFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	var files []adapter.FileMapping

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

	skillMappings, err := a.prepareSkillMappings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, skillMappings...)

	// Extended skills from content/skills/ via transformer
	extSkillMappings, err := a.renderExtendedSkills()
	if err != nil {
		return nil, fmt.Errorf("extended skill 준비 실패: %w", err)
	}
	files = append(files, extSkillMappings...)

	cmdMappings, err := a.prepareCommandMappings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, cmdMappings...)

	ruleMappings, err := a.prepareRuleMappings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, ruleMappings...)

	if cfg.IsFullMode() {
		agentMappings, err := a.prepareAgentMappings()
		if err != nil {
			return nil, err
		}
		files = append(files, agentMappings...)
	}

	settingsMappings, err := a.generateSettings(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, settingsMappings...)

	return files, nil
}


