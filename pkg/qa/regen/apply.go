package regen

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"gopkg.in/yaml.v3"
)

// ApplyOutcome reports the result of persisting one pack.
type ApplyOutcome struct {
	JourneyID string `json:"journey_id"`
	Path      string `json:"path,omitempty"`
	Written   bool   `json:"written"`
	Excluded  bool   `json:"excluded"`
	Reason    string `json:"reason,omitempty"`
}

// ApplyResult is the aggregate of an ApplyPacks call.
type ApplyResult struct {
	Written  []ApplyOutcome `json:"written"`
	Excluded []ApplyOutcome `json:"excluded"`
}

// ApplyPacks persists each accepted pack to
// <projectDir>/.autopus/qa/journeys/<id>.yaml. It re-validates every pack with
// journey.Validate and skips+reports any that fail (no partial file is written
// for a failing pack). It also re-runs the AI-authority guard so a web/desktop
// AI-authority pack is never persisted even though journey.Validate permits it.
//
// This is a pure write function: approval gating lives in Unit 2. Callers MUST
// only invoke this with packs the operator approved. Packs are guarded again
// here purely as a fail-closed safety net.
func ApplyPacks(projectDir string, packs []journey.Pack) (ApplyResult, error) {
	var result ApplyResult
	dir := filepath.Join(projectDir, ".autopus", "qa", "journeys")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result, err
	}
	for _, pack := range packs {
		if err := journey.Validate(pack, projectDir); err != nil {
			result.Excluded = append(result.Excluded, ApplyOutcome{
				JourneyID: pack.ID,
				Excluded:  true,
				Reason:    validationReason(err),
			})
			continue
		}
		if code, allowed := AIAuthorityGuard(pack); !allowed {
			result.Excluded = append(result.Excluded, ApplyOutcome{
				JourneyID: pack.ID,
				Excluded:  true,
				Reason:    code,
			})
			continue
		}
		// Defense in depth: a journey id is the persisted file name, so it must
		// be a single safe path segment. Reject any id that could traverse out
		// of the journeys directory (e.g. "../escape") instead of sanitizing it,
		// because the id is the pack's canonical identity and must not be
		// silently rewritten. journey.Validate only checks the id is non-empty.
		if !isSafeJourneyID(pack.ID) {
			result.Excluded = append(result.Excluded, ApplyOutcome{
				JourneyID: pack.ID,
				Excluded:  true,
				Reason:    "unsafe_journey_id",
			})
			continue
		}
		path := filepath.Join(dir, pack.ID+".yaml")
		body, err := yaml.Marshal(pack)
		if err != nil {
			return result, err
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return result, err
		}
		result.Written = append(result.Written, ApplyOutcome{
			JourneyID: pack.ID,
			Path:      path,
			Written:   true,
		})
	}
	return result, nil
}

// isSafeJourneyID reports whether id is a single safe file-name segment so that
// filepath.Join(dir, id+".yaml") cannot escape the journeys directory. It
// rejects empty ids, path separators, parent references, and any id whose base
// name differs from itself.
func isSafeJourneyID(id string) bool {
	if id == "" {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return false
	}
	return filepath.Base(id) == id
}
