package evidence

import (
	"fmt"
	"os"
	"path/filepath"
)

// Reasons an artifact is described in the bundle but not copied into it.
const (
	withheldLocalOnly   = "local_only_evidence"
	withheldNonText     = "non_text_media"
	withheldUnresolved  = "unresolved_artifact"
	bundleArtifactsDir  = "artifacts"
	bundleSchemaVersion = "qamesh.feedback.v2"
)

// bundleArtifact is one manifest artifact as it appears in a feedback bundle.
// BundleRel is set only when the bytes were copied into the bundle, so a
// consumer holding the bundle directory alone can resolve every path it sees.
type bundleArtifact struct {
	Ref       ArtifactRef
	BundleRel string
	Withheld  string
	Body      string
}

// exportBundleArtifacts copies publishable, redacted text artifacts into the
// bundle. A bundle is handed to another agent, possibly on another machine, so
// recording the producing run directory would only move the dangling reference;
// the bytes have to travel. Local-only refs and non-text media never travel:
// they are the exact evidence class that must stay on the producing machine.
func exportBundleArtifacts(manifest Manifest, bundlePath string) ([]bundleArtifact, error) {
	out := make([]bundleArtifact, 0, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		exported, err := exportBundleArtifact(manifest, artifact, bundlePath)
		if err != nil {
			return nil, err
		}
		out = append(out, exported)
	}
	return out, nil
}

func exportBundleArtifact(manifest Manifest, artifact ArtifactRef, bundlePath string) (bundleArtifact, error) {
	if !artifact.Publishable {
		return bundleArtifact{Ref: artifact, Withheld: withheldLocalOnly}, nil
	}
	if !isTextArtifactPath(artifact.Path) {
		return bundleArtifact{Ref: artifact, Withheld: withheldNonText}, nil
	}
	source, ok := resolveArtifactSource(manifest, artifact.Path)
	if !ok {
		return bundleArtifact{Ref: artifact, Withheld: withheldUnresolved}, nil
	}
	body, err := os.ReadFile(source)
	if err != nil {
		// A pruned or rotated run directory is a stale reference, not a safety
		// failure: describe the artifact instead of failing the repair loop.
		return bundleArtifact{Ref: artifact, Withheld: withheldUnresolved}, nil
	}
	// Re-running redaction is deliberate. The published artifact was redacted by
	// the redactor of the run that produced it; copying widens the blast radius,
	// so the bundle re-redacts and fails closed rather than trusting that pass.
	redacted := RedactText(string(body))
	rel := filepath.Join(bundleArtifactsDir, safePathSegment(artifact.Kind), safePathSegment(filepath.Base(source)))
	target := filepath.Join(bundlePath, rel)
	if err := AssertSafeText(redacted, rel); err != nil {
		return bundleArtifact{}, fmt.Errorf("feedback bundle artifact %s: %w", RedactText(rel), err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return bundleArtifact{}, err
	}
	if err := os.WriteFile(target, []byte(redacted), 0o644); err != nil {
		return bundleArtifact{}, err
	}
	return bundleArtifact{Ref: artifact, BundleRel: filepath.ToSlash(rel), Body: redacted}, nil
}

// resolveArtifactSource turns a manifest artifact path into a readable path.
// Manifest-relative paths resolve against the manifest's own directory, and the
// result must stay inside it: a manifest is untrusted input, so `../../secrets`
// must not become a bundled artifact.
func resolveArtifactSource(manifest Manifest, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	if manifest.sourceDir == "" {
		return path, filepath.IsAbs(path)
	}
	base, err := realAbsPath(manifest.sourceDir)
	if err != nil {
		return "", false
	}
	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(base, candidate)
	}
	resolved, err := realAbsPath(candidate)
	if err != nil || !isPathWithin(resolved, base) {
		return "", false
	}
	return resolved, true
}

func artifactSummaries(artifacts []bundleArtifact) []map[string]any {
	out := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		summary := map[string]any{
			"kind":        artifact.Ref.Kind,
			"publishable": artifact.Ref.Publishable,
			"redaction":   artifact.Ref.Redaction,
			"bundled":     artifact.BundleRel != "",
		}
		if artifact.BundleRel != "" {
			summary["path"] = artifact.BundleRel
		} else {
			// No `path` key: a reference a consumer cannot resolve is what made
			// the previous bundle format lie about being self-sufficient.
			summary["withheld"] = artifact.Withheld
		}
		out = append(out, summary)
	}
	return out
}
