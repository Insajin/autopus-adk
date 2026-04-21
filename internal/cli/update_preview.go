package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

type updatePreviewResult struct {
	Hint  string
	Items []previewItem
}

func cloneHarnessConfig(cfg *config.HarnessConfig) (*config.HarnessConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config clone: %w", err)
	}

	var clone config.HarnessConfig
	if err := yaml.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("unmarshal config clone: %w", err)
	}
	return &clone, nil
}

func buildUpdatePreview(ctx context.Context, dir string, cfg *config.HarnessConfig) (*updatePreviewResult, error) {
	result := &updatePreviewResult{Hint: describeRepoAwareHint(dir)}
	backupNeeded := false

	for _, platform := range cfg.Platforms {
		items, needsBackup, err := buildPlatformPreview(ctx, dir, cfg, platform)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, items...)
		backupNeeded = backupNeeded || needsBackup
	}

	if backupNeeded {
		result.Items = append(result.Items, previewItem{
			Path:     ".autopus/backup/<timestamp>/",
			Kind:     "preserve",
			Category: "runtime_state",
			Reason:   "user-modified files would be backed up before overwrite",
		})
	}

	return result, nil
}

func buildPlatformPreview(ctx context.Context, dir string, cfg *config.HarnessConfig, platform string) ([]previewItem, bool, error) {
	tempRoot, err := os.MkdirTemp("", "autopus-update-preview-*")
	if err != nil {
		return nil, false, fmt.Errorf("preview temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempRoot) }()

	files, err := generatePreviewMappings(ctx, tempRoot, cfg, platform)
	if err != nil {
		return nil, false, err
	}

	manifest, err := adapter.LoadManifest(dir, platform)
	if err != nil {
		return nil, false, err
	}

	items := make([]previewItem, 0, len(files)+1)
	needsBackup := false
	willWrite := false
	for _, file := range files {
		action := adapter.ResolveAction(dir, file.TargetPath, file.OverwritePolicy, manifest)
		items = append(items, previewItem{
			Path:     file.TargetPath,
			Kind:     previewKindForAction(action),
			Category: previewCategoryForPath(file.TargetPath),
			Reason:   reasonForUpdateAction(action),
			Scope:    platform,
		})
		if action == adapter.ActionBackup {
			needsBackup = true
		}
		if action != adapter.ActionSkip {
			willWrite = true
		}
	}

	if willWrite {
		kind := "create"
		if manifest != nil {
			kind = "update"
		}
		items = append(items, previewItem{
			Path:     filepath.ToSlash(filepath.Join(".autopus", platform+"-manifest.json")),
			Kind:     kind,
			Category: "runtime_state",
			Reason:   "managed file manifest would be refreshed after apply",
			Scope:    platform,
		})
	}

	return items, needsBackup, nil
}

func generatePreviewMappings(ctx context.Context, root string, cfg *config.HarnessConfig, platform string) ([]adapter.FileMapping, error) {
	switch platform {
	case "claude-code":
		pf, err := claude.NewWithRoot(root).Generate(ctx, cfg)
		return previewMappings(platform, pf, err)
	case "codex":
		pf, err := codex.NewWithRoot(root).Generate(ctx, cfg)
		return previewMappings(platform, pf, err)
	case "gemini-cli":
		pf, err := gemini.NewWithRoot(root).Generate(ctx, cfg)
		return previewMappings(platform, pf, err)
	case "opencode":
		pf, err := opencode.NewWithRoot(root).Generate(ctx, cfg)
		return previewMappings(platform, pf, err)
	default:
		return nil, fmt.Errorf("알 수 없는 플랫폼 %q", platform)
	}
}

func previewMappings(platform string, pf *adapter.PlatformFiles, err error) ([]adapter.FileMapping, error) {
	if err != nil {
		return nil, fmt.Errorf("%s preview 준비 실패: %w", platform, err)
	}
	if pf == nil {
		return nil, nil
	}
	return pf.Files, nil
}

func previewKindForAction(action adapter.UpdateAction) string {
	switch action {
	case adapter.ActionCreate:
		return "create"
	case adapter.ActionSkip:
		return "skip"
	case adapter.ActionBackup:
		return "preserve"
	default:
		return "update"
	}
}

func reasonForUpdateAction(action adapter.UpdateAction) string {
	switch action {
	case adapter.ActionCreate:
		return "new managed file would be created"
	case adapter.ActionSkip:
		return "locally deleted managed file would remain untouched"
	case adapter.ActionBackup:
		return "existing file would be preserved via backup before overwrite"
	default:
		return "managed file can be refreshed in place"
	}
}
