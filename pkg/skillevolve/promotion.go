package skillevolve

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

var requiredPromotionChecks = []string{"template_parity", "go test ./..."}

type promotionTarget struct {
	Rel     string
	Path    string
	Content string
}

type fileSnapshot struct {
	Exists bool
	Mode   fs.FileMode
	Body   []byte
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-SKILL-EVOLVE-001: promotion is the only path that applies replay-approved candidates to ADK source of truth.
// @AX:REASON: Human approval, static safety, source path policy, and required post-apply checks must remain coupled at this boundary.
func PromoteCandidate(ctx context.Context, opts PromotionOptions) (PromotionResult, error) {
	if err := ctx.Err(); err != nil {
		return PromotionResult{}, err
	}
	result := PromotionResult{RequiredChecks: append([]string{}, requiredPromotionChecks...)}
	if !opts.Candidate.PromotionReady || opts.Candidate.Status != "promotion_ready" {
		return result, errors.New("candidate is not promotion_ready")
	}
	if !hasHumanApproval(opts.Approval) {
		return result, errors.New("human approval is required")
	}
	if len(opts.Candidate.ProposedFiles) == 0 {
		return result, errors.New("candidate has no proposed files")
	}

	safety, err := EvaluateSafety(ctx, opts.Candidate, SafetyOptions{})
	if err != nil {
		return result, err
	}
	if !safety.PromotionAllowed {
		return result, errors.New("candidate failed static safety gate")
	}
	targets, err := promotionTargets(opts.ProjectDir, opts.Candidate)
	if err != nil {
		return result, err
	}
	if !opts.Apply {
		return result, nil
	}

	if err := applyPromotionTargets(targets); err != nil {
		return result, err
	}
	applied := make([]string, 0, len(targets))
	for _, target := range targets {
		applied = append(applied, target.Rel)
	}
	sort.Strings(applied)
	result.Applied = true
	result.AppliedPaths = applied
	return result, nil
}

func promotionTargets(projectDir string, candidate CandidateBundle) ([]promotionTarget, error) {
	if projectDir == "" {
		projectDir = "."
	}
	targets := make([]promotionTarget, 0, len(candidate.ProposedFiles))
	seen := make(map[string]struct{}, len(candidate.ProposedFiles))
	for _, file := range candidate.ProposedFiles {
		if !isADKSourceOfTruthPath(file.Path) {
			return nil, errors.New("candidate proposes non-ADK source-of-truth path")
		}
		rel := cleanRelPath(file.Path)
		if _, exists := seen[rel]; exists {
			return nil, fmt.Errorf("candidate proposes duplicate target %q", rel)
		}
		seen[rel] = struct{}{}
		target := filepath.Join(projectDir, filepath.FromSlash(rel))
		if !pathWithinProjectForWrite(projectDir, target) {
			return nil, errors.New("candidate target escapes project directory")
		}
		if info, err := os.Lstat(target); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, errors.New("candidate target uses symlink")
			}
			if info.IsDir() {
				return nil, errors.New("candidate target is a directory")
			}
		}
		targets = append(targets, promotionTarget{
			Rel:     rel,
			Path:    target,
			Content: file.Content,
		})
	}
	return targets, nil
}

func applyPromotionTargets(targets []promotionTarget) error {
	snapshots, err := snapshotPromotionTargets(targets)
	if err != nil {
		return err
	}
	applied := make([]promotionTarget, 0, len(targets))
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target.Path), 0o755); err != nil {
			rollbackPromotionTargets(applied, snapshots)
			return err
		}
		if err := writePromotionTarget(target); err != nil {
			rollbackPromotionTargets(applied, snapshots)
			return err
		}
		applied = append(applied, target)
	}
	return nil
}

func snapshotPromotionTargets(targets []promotionTarget) (map[string]fileSnapshot, error) {
	snapshots := make(map[string]fileSnapshot, len(targets))
	for _, target := range targets {
		info, err := os.Lstat(target.Path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				snapshots[target.Path] = fileSnapshot{}
				continue
			}
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("candidate target uses symlink")
		}
		body, err := os.ReadFile(target.Path)
		if err != nil {
			return nil, err
		}
		snapshots[target.Path] = fileSnapshot{
			Exists: true,
			Mode:   info.Mode().Perm(),
			Body:   body,
		}
	}
	return snapshots, nil
}

func writePromotionTarget(target promotionTarget) error {
	tmp, err := os.CreateTemp(filepath.Dir(target.Path), ".autopus-skill-evolve-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write([]byte(target.Content)); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, target.Path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func rollbackPromotionTargets(targets []promotionTarget, snapshots map[string]fileSnapshot) {
	for i := len(targets) - 1; i >= 0; i-- {
		target := targets[i]
		snapshot := snapshots[target.Path]
		if snapshot.Exists {
			mode := snapshot.Mode
			if mode == 0 {
				mode = 0o644
			}
			_ = os.WriteFile(target.Path, snapshot.Body, mode)
			continue
		}
		_ = os.Remove(target.Path)
	}
}

func hasHumanApproval(approval HumanApproval) bool {
	return approval.Approved || approval.ApprovedBy != ""
}
