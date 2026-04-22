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

	result.Items = append(result.Items, buildStatusLinePreviewItems(dir, cfg)...)

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
	diff := adapter.BuildManifestDiff(manifest, files, previewPruneRoots(platform))
	for _, entry := range diff.Emit {
		file, ok := lookupPreviewFile(files, entry.Path)
		if !ok {
			continue
		}
		action := adapter.ResolveAction(dir, file.TargetPath, file.OverwritePolicy, manifest)
		kind := adapter.ManifestActionEmit
		reason := reasonForManifestEntry(entry, action)
		if action == adapter.ActionSkip {
			kind = adapter.ManifestActionRetain
		}
		items = append(items, previewItem{
			Path:     entry.Path,
			Kind:     kind,
			Category: previewCategoryForPath(entry.Path),
			Reason:   reason,
			Scope:    platform,
		})
		if action == adapter.ActionBackup {
			needsBackup = true
		}
		if action != adapter.ActionSkip {
			willWrite = true
		}
	}
	for _, entry := range diff.Retain {
		file, ok := lookupPreviewFile(files, entry.Path)
		if !ok {
			continue
		}
		action := adapter.ResolveAction(dir, file.TargetPath, file.OverwritePolicy, manifest)
		if action == adapter.ActionBackup {
			needsBackup = true
			willWrite = true
		}
		items = append(items, previewItem{
			Path:     entry.Path,
			Kind:     adapter.ManifestActionRetain,
			Category: previewCategoryForPath(entry.Path),
			Reason:   reasonForManifestEntry(entry, action),
			Scope:    platform,
		})
	}
	for _, entry := range diff.Prune {
		items = append(items, previewItem{
			Path:     entry.Path,
			Kind:     adapter.ManifestActionPrune,
			Category: previewCategoryForPath(entry.Path),
			Reason:   reasonForManifestEntry(entry, adapter.ActionOverwrite),
			Scope:    platform,
		})
		willWrite = true
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
			Reason:   manifestDiffSummary(diff),
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

func lookupPreviewFile(files []adapter.FileMapping, path string) (adapter.FileMapping, bool) {
	for _, file := range files {
		if filepath.ToSlash(file.TargetPath) == path {
			return file, true
		}
	}
	return adapter.FileMapping{}, false
}

func previewPruneRoots(platform string) []string {
	switch platform {
	case "codex":
		return []string{".codex/skills", filepath.ToSlash(filepath.Join(".autopus", "plugins", "auto", "skills"))}
	case "opencode":
		return []string{".agents/skills", ".opencode/skills"}
	default:
		return nil
	}
}

func manifestDiffSummary(diff adapter.ManifestDiff) string {
	return fmt.Sprintf(
		"manifest diff would record %d emit, %d retain, %d prune actions",
		len(diff.Emit),
		len(diff.Retain),
		len(diff.Prune),
	)
}

func reasonForManifestEntry(entry adapter.ManifestDiffEntry, action adapter.UpdateAction) string {
	switch entry.Action {
	case adapter.ManifestActionRetain:
		if action == adapter.ActionBackup {
			return "managed checksum is unchanged, but local edits would be backed up before the file is refreshed"
		}
		if action == adapter.ActionSkip {
			return "managed checksum is unchanged and the locally deleted file would remain untouched"
		}
		return "managed checksum is unchanged and would be retained"
	case adapter.ManifestActionPrune:
		return "stale managed artifact would be pruned from the compiler-owned surface"
	default:
		if action == adapter.ActionSkip {
			return "locally deleted managed file would remain untouched"
		}
		if action == adapter.ActionBackup {
			return "managed output would emit with checksum diff after backing up local edits"
		}
		if entry.OldChecksum == "" {
			return "new managed output would emit into the target surface"
		}
		return "managed output would emit with checksum diff"
	}
}
