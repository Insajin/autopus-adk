package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ErrStaleChangePlan reports that the filesystem no longer matches the preview.
var ErrStaleChangePlan = errors.New("stale change plan")

// ApplyChangePlan revalidates and writes the files described by a preview plan.
func ApplyChangePlan(plan *ChangePlan) (*ApplyResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("apply change plan: nil plan")
	}
	if err := revalidateChangePlan(plan); err != nil {
		return nil, err
	}

	changedPaths := make([]string, 0, len(plan.targets))
	for _, target := range plan.targets {
		switch target.change.Action {
		case ChangeActionCreate, ChangeActionUpdate:
			if err := os.MkdirAll(filepath.Dir(target.absPath), 0755); err != nil {
				return nil, fmt.Errorf("write %s: %w", target.change.Path, err)
			}
			if err := os.WriteFile(target.absPath, target.content, 0644); err != nil {
				return nil, fmt.Errorf("write %s: %w", target.change.Path, err)
			}
			changedPaths = append(changedPaths, target.change.Path)
		}
	}

	sort.Strings(changedPaths)
	return &ApplyResult{
		ChangedPaths: changedPaths,
		DocSet:       plan.docSet,
	}, nil
}

func revalidateChangePlan(plan *ChangePlan) error {
	current, err := rebuildChangePlan(plan)
	if err != nil {
		return err
	}
	if current.Fingerprint == plan.Fingerprint {
		return nil
	}
	return fmt.Errorf("%w: preview no longer matches current filesystem", ErrStaleChangePlan)
}

func rebuildChangePlan(plan *ChangePlan) (*ChangePlan, error) {
	switch plan.Mode {
	case ChangePlanModeGenerate:
		opts := plan.generateOpts
		opts.Force = true
		return buildGeneratePlanAt(plan.ProjectDir, &opts, plan.BuiltAt)
	case ChangePlanModeUpdate:
		return buildUpdatePlanAt(plan.ProjectDir, plan.generateOpts.OutputDir, plan.BuiltAt)
	default:
		return nil, fmt.Errorf("rebuild change plan: unsupported mode %q", plan.Mode)
	}
}
