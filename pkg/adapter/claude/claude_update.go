package claude

import (
	"context"
	"fmt"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// PruneRoots are the compiler-owned directories an update may delete stale
// managed files from. It is the single source of truth for both the apply path
// below and the `auto update` preview, which must agree: a preview that
// promises a prune the apply path cannot perform leaves orphaned files behind
// forever, because the manifest is rewritten either way and never proposes the
// prune again.
//
// Only compiler-owned directories belong here. Pruning is additionally limited
// to paths recorded in the previous manifest (see BuildManifestDiff), so a file
// the user created in one of these directories is never eligible.
//
// The rule directory and the conditional body root are both listed because
// SPEC-CONDRULE-001 moves a hook-fired rule body between them: relocating out
// of `.claude/rules/autopus` must delete the stale baseline copy, and a rule
// reclassified back to `always` must delete the stale relocated body.
func PruneRoots() []string {
	return []string{
		".claude/skills/autopus",
		rulecond.ClaudeRulesRelDir,
		rulecond.BodyRootRelPath,
	}
}

// Update는 매니페스트 기반으로 파일을 업데이트한다.
// 사용자가 수정한 파일은 백업 후 덮어쓰고, 삭제한 파일은 재생성하지 않는다.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	newFiles, err := a.prepareFiles(cfg)
	if err != nil {
		return nil, err
	}
	hookFiles, err := a.prepareHooksAndPermissionsFiles(cfg)
	if err != nil {
		return nil, err
	}
	newFiles = append(newFiles, hookFiles...)

	// REQ-004: keep Update in parity with Generate — the generated Route A
	// workflow JS is a full-mode surface, so it must be re-emitted on update too.
	if cfg.IsFullMode() {
		workflowFiles, err := a.workflowFiles(cfg)
		if err != nil {
			return nil, err
		}
		newFiles = append(newFiles, workflowFiles...)
	}

	plan, pf := a.buildUpdateTransactionPlan(oldManifest, newFiles)
	if _, err := adapter.ApplyTransaction(a.root, adapterName, plan); err != nil {
		return nil, err
	}

	return pf, nil
}

func (a *Adapter) buildUpdateTransactionPlan(
	oldManifest *adapter.Manifest,
	newFiles []adapter.FileMapping,
) (adapter.TransactionPlan, *adapter.PlatformFiles) {
	finalFiles := make([]adapter.FileMapping, 0, len(newFiles))
	skippedPaths := make([]string, 0)

	for _, file := range newFiles {
		action := adapter.ResolveAction(a.root, file.TargetPath, file.OverwritePolicy, oldManifest)
		if action == adapter.ActionSkip {
			skippedPaths = append(skippedPaths, file.TargetPath)
			continue
		}
		finalFiles = append(finalFiles, file)
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: checksum(fmt.Sprintf("%d", len(finalFiles))),
	}
	manifest := adapter.ManifestFromFiles(adapterName, pf)
	if oldManifest != nil {
		for _, path := range skippedPaths {
			if prev, ok := oldManifest.Files[path]; ok {
				manifest.Files[path] = prev
			}
		}
	}
	diff := adapter.BuildManifestDiff(oldManifest, newFiles, PruneRoots())

	return adapter.TransactionPlan{
		Writes:   adapter.TransactionWritesFromFiles(finalFiles, claudeFileMode),
		Removes:  adapter.TransactionRemovesFromManifestDiff(diff, false),
		Manifest: manifest,
	}, pf
}
