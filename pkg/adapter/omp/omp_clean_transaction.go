package omp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// CleanReceipt records paths changed before a later caller-side failure.
type CleanReceipt struct {
	ChangedPaths []string
}

type ompCleanStep struct {
	relPath     string
	previous    adapter.ManifestFile
	markerBody  []byte
	marker      bool
	perm        os.FileMode
	removeWhole bool
	needsBackup bool
	restore     bool
	checksum    string
}

type ompCleanPlan struct {
	steps            []ompCleanStep
	manifestRel      string
	manifestChecksum string
	project          *ompProjectCleanState
}

// CleanWithReceipt validates every destructive path before applying the plan.
// The returned receipt lets the CLI describe a repairable late config failure.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-002: downgraded from ANCHOR because fan-in is below 3.
// This is the transactional destructive boundary for OMP removal: all paths are proven safe before mutation, and callers use
// ChangedPaths to distinguish a preflight failure from a repair-required partial removal.
func (a *Adapter) CleanWithReceipt(_ context.Context) (receipt CleanReceipt, returnErr error) {
	workspace, err := openOMPRootedWorkspace(a.root)
	if err != nil {
		return CleanReceipt{}, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, workspace.Close()) }()
	plan, err := a.prepareCleanAt(workspace)
	if err != nil {
		return CleanReceipt{}, err
	}
	return a.applyCleanAt(workspace, plan)
}

// @AX:WARN [AUTO]: transactional cleanup planning has cyclomatic complexity 19.
// @AX:REASON [AUTO]: gocyclo reports 19 across manifest trust, ownership, checksum, path, and preimage checks.
func (a *Adapter) prepareCleanAt(workspace *ompRootedWorkspace) (ompCleanPlan, error) {
	manifest, err := loadOMPManifestAt(workspace, adapterName)
	if err != nil || manifest == nil {
		return ompCleanPlan{}, err
	}
	plan, err := a.prepareCleanManifestAt(workspace)
	if err != nil {
		return ompCleanPlan{}, err
	}
	plan.project, err = a.prepareOMPProjectCleanStateAt(workspace)
	if err != nil {
		return ompCleanPlan{}, err
	}

	roots := a.cleanPruneRoots()
	paths := make([]string, 0, len(manifest.Files))
	for path := range manifest.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		previous := manifest.Files[path]
		if strings.HasPrefix(path, filepath.Join(".git", "hooks")) || !isPruneEligible(path, roots) {
			continue
		}
		if _, pathErr := cleanOMPRootedPath(path); pathErr != nil {
			return ompCleanPlan{}, fmt.Errorf("%s 안전 경로 확인 실패: %w", path, pathErr)
		}
		data, info, readErr := workspace.readFile(path, 4<<20)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return ompCleanPlan{}, fmt.Errorf("%s 읽기 실패: %w", path, readErr)
		}
		if !validOMPManifestChecksum(previous.Checksum) {
			return ompCleanPlan{}, fmt.Errorf("%s 매니페스트 checksum이 올바르지 않습니다", path)
		}

		step := ompCleanStep{
			relPath: path, previous: previous, removeWhole: true,
			checksum: adapter.Checksum(string(data)), perm: info.Mode().Perm(),
			needsBackup: previous.Policy != adapter.OverwriteMarker &&
				adapter.Checksum(string(data)) != previous.Checksum,
		}
		if filepath.ToSlash(path) == configFile && plan.project != nil {
			step.restore = !plan.project.missing
			step.markerBody = append([]byte(nil), plan.project.preimage...)
			step.perm = plan.project.mode
			step.removeWhole = plan.project.missing
			step.needsBackup = false
			plan.steps = append(plan.steps, step)
			continue
		}
		if previous.Policy == adapter.OverwriteMarker {
			remainder, found, stripErr := stripOMPManagedDocument(string(data))
			if stripErr != nil {
				return ompCleanPlan{}, stripErr
			}
			if !found {
				continue
			}
			step.marker = true
			step.markerBody = []byte(remainder)
			step.perm = info.Mode().Perm()
			step.removeWhole = strings.TrimSpace(remainder) == ""
		}
		plan.steps = append(plan.steps, step)
	}

	return plan, nil
}

func (a *Adapter) prepareCleanManifestAt(workspace *ompRootedWorkspace) (ompCleanPlan, error) {
	plan := ompCleanPlan{manifestRel: filepath.Join(".autopus", adapterName+"-manifest.json")}
	data, _, err := workspace.readFile(plan.manifestRel, 4<<20)
	if err != nil {
		return ompCleanPlan{}, fmt.Errorf("%s 읽기 실패: %w", plan.manifestRel, err)
	}
	plan.manifestChecksum = adapter.Checksum(string(data))
	return plan, nil
}

func (a *Adapter) applyCleanAt(workspace *ompRootedWorkspace, plan ompCleanPlan) (CleanReceipt, error) {
	return a.applyCleanAtWithHook(workspace, plan, nil)
}

func validOMPManifestChecksum(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func stripOMPManagedDocument(existing string) (string, bool, error) {
	layout, err := parseOMPConfigLayout(existing)
	if err != nil {
		return "", false, err
	}
	span, found, err := findOMPMarkerSpan(existing, splitOMPRawLines(existing))
	if err != nil {
		return "", false, err
	}
	if !found {
		if layout.customKey != nil {
			return "", false, fmt.Errorf("%s의 skills.customDirectories에 관리 마커가 없습니다", configFile)
		}
		return existing, false, nil
	}
	if err := validateManagedOMPSpan(layout, span); err != nil {
		return "", false, err
	}
	remainder := existing[:span.start] + existing[span.end:]
	return remainder, true, nil
}
