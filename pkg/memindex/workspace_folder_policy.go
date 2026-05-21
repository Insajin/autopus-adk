package memindex

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	WorkspaceFolderClassIndexable      = "indexable"
	WorkspaceFolderClassCandidate      = "candidate"
	WorkspaceFolderClassProjectionOnly = "projection_only"
	WorkspaceFolderClassExcluded       = "excluded"

	SkipReasonWorkspaceRuntime          = "workspace_runtime"
	SkipReasonWorkspaceRawArtifact      = "workspace_raw_artifact"
	SkipReasonWorkspaceContextSignature = "workspace_context_signature"
	SkipReasonWorkspaceManifest         = "workspace_manifest"
	SkipReasonWorkspaceGeneratedSurface = "workspace_generated_surface"
	SkipReasonWorkspaceHarnessSurface   = "workspace_harness_surface"
	SkipReasonWorkspaceConfig           = "workspace_config"
)

type WorkspaceFolderClassification struct {
	Class      string
	ReasonCode string
}

func classifyWorkspaceFolderPath(rel string) WorkspaceFolderClassification {
	rel = cleanWorkspaceRel(rel)
	switch {
	case rel == "readme.md":
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassIndexable}
	case rel == "docs" || strings.HasPrefix(rel, "docs/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassIndexable}
	case rel == ".autopus/project" || strings.HasPrefix(rel, ".autopus/project/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassIndexable}
	case rel == ".autopus/specs" || strings.HasPrefix(rel, ".autopus/specs/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassIndexable}
	case rel == ".autopus/vault" || strings.HasPrefix(rel, ".autopus/vault/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassIndexable}
	case rel == ".autopus/inbox" || strings.HasPrefix(rel, ".autopus/inbox/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassCandidate}
	case rel == ".autopus/learnings/pipeline.jsonl":
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassProjectionOnly}
	case rel == ".autopus/runtime" || strings.HasPrefix(rel, ".autopus/runtime/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceRuntime}
	case rel == ".autopus/qa" || strings.HasPrefix(rel, ".autopus/qa/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceRawArtifact}
	case rel == ".autopus/context/signatures.md":
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceContextSignature}
	case isAutopusManifest(rel):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceManifest}
	case rel == ".autopus/plugins" || strings.HasPrefix(rel, ".autopus/plugins/") ||
		rel == ".autopus/orchestra" || strings.HasPrefix(rel, ".autopus/orchestra/") ||
		rel == ".autopus/brainstorms" || strings.HasPrefix(rel, ".autopus/brainstorms/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceGeneratedSurface}
	case rel == ".codex" || strings.HasPrefix(rel, ".codex/") ||
		rel == ".claude" || strings.HasPrefix(rel, ".claude/") ||
		rel == ".gemini" || strings.HasPrefix(rel, ".gemini/") ||
		rel == ".opencode" || strings.HasPrefix(rel, ".opencode/") ||
		rel == ".agents/plugins" || strings.HasPrefix(rel, ".agents/plugins/") ||
		rel == ".symphony/artifacts" || strings.HasPrefix(rel, ".symphony/artifacts/"):
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceHarnessSurface}
	case rel == "config.toml":
		return WorkspaceFolderClassification{Class: WorkspaceFolderClassExcluded, ReasonCode: SkipReasonWorkspaceConfig}
	default:
		return WorkspaceFolderClassification{}
	}
}

func workspaceFolderPolicySkips(projectDir string) []Skip {
	candidates := []string{
		".autopus/runtime",
		".autopus/qa",
		".autopus/context/signatures.md",
		".autopus/plugins",
		".autopus/orchestra",
		".autopus/brainstorms",
		".codex",
		".claude",
		".gemini",
		".opencode",
		".agents/plugins",
		".symphony/artifacts",
		"config.toml",
	}
	skips := make([]Skip, 0, len(candidates))
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(projectDir, filepath.FromSlash(rel))); err == nil {
			classification := classifyWorkspaceFolderPath(rel)
			if classification.Class == WorkspaceFolderClassExcluded {
				skips = append(skips, Skip{Path: rel, Reason: classification.ReasonCode})
				if legacyGeneratedSurfaceReason(classification.ReasonCode) {
					skips = append(skips, Skip{Path: rel, Reason: "generated_surface"})
				}
			}
		}
	}
	manifestMatches, _ := filepath.Glob(filepath.Join(projectDir, ".autopus", "*-manifest.json"))
	for _, path := range manifestMatches {
		skips = append(skips, Skip{Path: slashRel(projectDir, path), Reason: SkipReasonWorkspaceManifest})
	}
	return skips
}

func workspaceMarkdownMetadata(classification WorkspaceFolderClassification) map[string]any {
	switch classification.Class {
	case WorkspaceFolderClassCandidate:
		return map[string]any{
			"candidate":               true,
			"promotion_required":      true,
			"canonical_knowledge_hub": false,
		}
	case WorkspaceFolderClassIndexable:
		return map[string]any{
			"candidate":               false,
			"canonical_knowledge_hub": false,
			"projection_only":         true,
		}
	default:
		return nil
	}
}

func cleanWorkspaceRel(rel string) string {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || rel == "." {
		return ""
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
}

func isAutopusManifest(rel string) bool {
	if !strings.HasPrefix(rel, ".autopus/") || strings.Count(rel, "/") != 1 {
		return false
	}
	return strings.HasSuffix(rel, "-manifest.json")
}

func legacyGeneratedSurfaceReason(reason string) bool {
	return reason == SkipReasonWorkspaceGeneratedSurface || reason == SkipReasonWorkspaceHarnessSurface
}
