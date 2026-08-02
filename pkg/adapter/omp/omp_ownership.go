package omp

import "github.com/insajin/autopus-adk/pkg/config"

// PruneRoots are the compiler-owned directories omp may delete stale
// manifest-recorded files from. It is the single source of truth for Update,
// Clean, and the `auto update` preview, which must agree: a preview that
// promises a prune the apply path cannot perform leaves orphaned files behind
// forever, because the manifest is rewritten either way and never proposes the
// prune again.
//
// Pruning is additionally limited to paths recorded in the previous manifest
// (see adapter.BuildManifestDiff and Clean), so a file omp never wrote is never
// eligible: antigravity's .agents/commands/*.toml and a user's own
// .agents/rules/*.md survive because omp never records them.
//
// Eligibility is decided per surface, not per operation. .agents/commands stays
// listed even after omp yields the command surface, because opencode serves
// commands from .opencode/commands/ — nothing ever overwrites the .md files omp
// wrote under .agents/commands, so dropping the root would strand all of them.
// .agents/skills is the opposite case: opencode and codex rewrite
// .agents/skills/auto/SKILL.md in place, so once omp yields that surface the
// file on disk belongs to them and omp must not delete it.
func PruneRoots(cfg *config.HarnessConfig) []string {
	roots := ompExclusivePruneRoots()
	if ompOwnsSharedSkillSurface(cfg) {
		roots = append(roots, ".agents/skills")
	}
	return roots
}

// ompExclusivePruneRoots are the surfaces omp owns no matter which other
// platforms are active. Clean falls back to this set when the harness config
// cannot be read: without the platform list, assuming omp still owns the shared
// skill surface would delete the .agents/skills/auto/SKILL.md that opencode or
// codex rewrote in place. Losing another platform's live file is worse than
// leaving an omp file behind, so unprovable ownership fails closed.
func ompExclusivePruneRoots() []string {
	return []string{
		".agents/rules/autopus",
		".omp/agents",
		configFile,
		".agents/commands",
		ompContextBridgeTarget,
		DefaultOMPModelOverlayPath,
		OMPModelReceiptRelativePath,
		OMPModelProjectOwnershipRelativePath,
	}
}

func ompOwnsSharedSkillSurface(cfg *config.HarnessConfig) bool {
	hasCodex := false
	hasOpenCode := false
	for _, p := range cfg.Platforms {
		if p == "codex" {
			hasCodex = true
		}
		if p == "opencode" {
			hasOpenCode = true
		}
	}
	return !hasCodex && !hasOpenCode
}

func ompOwnsCommandSurface(cfg *config.HarnessConfig) bool {
	hasOpenCode := false
	for _, p := range cfg.Platforms {
		if p == "opencode" {
			hasOpenCode = true
		}
	}
	return !hasOpenCode
}
