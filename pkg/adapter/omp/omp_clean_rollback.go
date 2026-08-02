package omp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

type ompCleanPreimage struct {
	path      string
	data      []byte
	mode      os.FileMode
	missing   bool
	directory bool
}

type ompCleanApplyHook func(path string) error

func (a *Adapter) applyCleanAtWithHook(
	workspace *ompRootedWorkspace,
	plan ompCleanPlan,
	hook ompCleanApplyHook,
) (CleanReceipt, error) {
	preimages, err := captureOMPCleanPlan(workspace, plan)
	if err != nil {
		return CleanReceipt{}, err
	}
	applied := make([]ompCleanPreimage, 0, len(plan.steps)+1)
	changed := make([]string, 0, len(plan.steps)+1)
	fail := func(cause error) (CleanReceipt, error) {
		if rollbackErr := rollbackOMPClean(workspace, applied); rollbackErr != nil {
			sort.Strings(changed)
			return CleanReceipt{ChangedPaths: changed}, errors.Join(cause, fmt.Errorf("clean rollback failed: %w", rollbackErr))
		}
		return CleanReceipt{}, cause
	}

	backupRoot := filepath.Join(".autopus", "backup", time.Now().UTC().Format("20060102T150405.000000000"))
	for _, step := range plan.steps {
		if !step.needsBackup {
			continue
		}
		backupPath := filepath.Join(backupRoot, step.relPath)
		backup, captureErr := captureOMPCleanPath(workspace, backupPath, true)
		if captureErr != nil {
			return fail(captureErr)
		}
		missingParents, parentErr := captureOMPCleanMissingParents(workspace, backupPath)
		if parentErr != nil {
			return fail(parentErr)
		}
		applied = append(applied, missingParents...)
		if copyErr := workspace.copyFile(step.relPath, backupPath, step.perm); copyErr != nil {
			return fail(copyErr)
		}
		applied = append(applied, backup)
	}

	prune := make([]string, 0, len(plan.steps))
	for _, step := range plan.steps {
		if hook != nil {
			if hookErr := hook(step.relPath); hookErr != nil {
				return fail(hookErr)
			}
		}
		if mutateErr := applyOMPCleanStep(workspace, step); mutateErr != nil {
			return fail(mutateErr)
		}
		applied = append(applied, preimages[step.relPath])
		changed = append(changed, filepath.ToSlash(step.relPath))
		if step.removeWhole {
			prune = append(prune, filepath.Dir(step.relPath))
		}
	}
	if hook != nil {
		if hookErr := hook(plan.manifestRel); hookErr != nil {
			return fail(hookErr)
		}
	}
	if err := workspace.remove(plan.manifestRel, false); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fail(fmt.Errorf("%s 제거 실패: %w", plan.manifestRel, err))
	}
	applied = append(applied, preimages[plan.manifestRel])
	changed = append(changed, filepath.ToSlash(plan.manifestRel))
	prune = append(prune, filepath.Dir(plan.manifestRel))

	for _, parent := range prune {
		removed, pruneErr := pruneOMPCleanParents(workspace, parent)
		applied = append(applied, removed...)
		if pruneErr != nil {
			return fail(pruneErr)
		}
	}
	sort.Strings(changed)
	return CleanReceipt{ChangedPaths: changed}, nil
}

func captureOMPCleanPlan(
	workspace *ompRootedWorkspace,
	plan ompCleanPlan,
) (map[string]ompCleanPreimage, error) {
	preimages := make(map[string]ompCleanPreimage, len(plan.steps)+1)
	for _, step := range plan.steps {
		preimage, err := captureOMPCleanPath(workspace, step.relPath, false)
		if err != nil || adapter.Checksum(string(preimage.data)) != step.checksum {
			return nil, fmt.Errorf("%s가 preflight 이후 변경되었습니다", step.relPath)
		}
		preimages[step.relPath] = preimage
	}
	manifest, err := captureOMPCleanPath(workspace, plan.manifestRel, false)
	if err != nil || adapter.Checksum(string(manifest.data)) != plan.manifestChecksum {
		return nil, fmt.Errorf("%s가 preflight 이후 변경되었습니다", plan.manifestRel)
	}
	preimages[plan.manifestRel] = manifest
	return preimages, nil
}

func captureOMPCleanPath(
	workspace *ompRootedWorkspace,
	path string,
	allowMissing bool,
) (ompCleanPreimage, error) {
	data, info, err := workspace.readFile(path, 4<<20)
	if allowMissing && errors.Is(err, fs.ErrNotExist) {
		return ompCleanPreimage{path: path, missing: true}, nil
	}
	if err != nil {
		return ompCleanPreimage{}, err
	}
	return ompCleanPreimage{path: path, data: data, mode: info.Mode().Perm()}, nil
}

func applyOMPCleanStep(workspace *ompRootedWorkspace, step ompCleanStep) error {
	if step.restore || step.marker && !step.removeWhole {
		if err := workspace.atomicWrite(step.relPath, step.markerBody, step.perm); err != nil {
			return fmt.Errorf("%s 쓰기 실패: %w", step.relPath, err)
		}
		return nil
	}
	if err := workspace.remove(step.relPath, false); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s 제거 실패: %w", step.relPath, err)
	}
	return nil
}
