package cli

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/gemini"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
)

// maxDriftReprPaths caps the representative drift paths surfaced per platform so
// the advisory output stays compact even when many files are stale (REQ-001).
const maxDriftReprPaths = 3

// contentDriftResult is one platform's installed-surface content-drift verdict.
// Compared counts the deterministic files that were checked; DriftCount is the
// number that differ from the current binary's generation; DriftPaths holds up
// to maxDriftReprPaths representative target paths for the report.
type contentDriftResult struct {
	Platform   string
	Compared   int
	DriftCount int
	DriftPaths []string
}

// driftAdapterFor returns the platform adapter rooted at root, or nil for an
// unknown platform token. It mirrors validateDoctorPlatform's switch so the
// drift gate reuses the same adapter contract the rest of doctor relies on.
func driftAdapterFor(platform, root string) adapter.PlatformAdapter {
	switch platform {
	case "claude-code":
		return claude.NewWithRoot(root)
	case "codex":
		return codex.NewWithRoot(root)
	case "antigravity-cli":
		return gemini.NewWithRoot(root)
	case "opencode":
		return opencode.NewWithRoot(root)
	default:
		return nil
	}
}

// collectContentDrift compares each configured platform's installed deterministic
// surface against what the current binary would generate. Only platforms with an
// installed manifest are checked; others are silently skipped (REQ-008). The
// installed surface and real manifests are never mutated — generation happens in
// isolated temp roots.
func collectContentDrift(dir string, cfg *config.HarnessConfig) []contentDriftResult {
	if cfg == nil {
		return nil
	}
	var results []contentDriftResult
	for _, platform := range cfg.Platforms {
		// Manifest presence is the "installed" gate. A read error or absent
		// manifest means this platform is not installed here — skip quietly.
		m, err := adapter.LoadManifest(dir, platform)
		if err != nil || m == nil {
			continue
		}
		res, ok := computeContentDrift(dir, platform, cfg)
		if !ok {
			continue
		}
		results = append(results, res)
	}
	return results
}

// computeContentDrift derives the deterministic template+cfg subset for a
// platform and hashes each installed file against the freshly generated content.
// It returns ok=false when generation into a temp root fails, so a broken
// platform degrades to a silent skip rather than a false drift signal.
func computeContentDrift(dir, platform string, cfg *config.HarnessConfig) (contentDriftResult, bool) {
	// Two isolated seeded temp roots isolate the pure template+cfg surface from
	// root-path and pre-existing-state dependent files (F-002 determinism gate):
	// A is empty, B carries representative user state. Only files byte-identical
	// across A and B are pure functions of (template, cfg) and safe to compare.
	filesA, ok := generateDriftBaseline(platform, cfg, nil)
	if !ok {
		return contentDriftResult{}, false
	}
	filesB, ok := generateDriftBaseline(platform, cfg, seedUserState)
	if !ok {
		return contentDriftResult{}, false
	}

	deterministic := deterministicOverwriteSubset(filesA, filesB)

	res := contentDriftResult{Platform: platform}
	var drifted []string
	for target, wantChecksum := range deterministic {
		installed, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(target)))
		if err != nil {
			// A missing installed file is a tracking concern owned by the git
			// hygiene check, not content staleness. Skip it so the advisory
			// content gate never false-positives on a partial install.
			continue
		}
		res.Compared++
		if adapter.Checksum(string(installed)) != wantChecksum {
			drifted = append(drifted, target)
		}
	}

	sort.Strings(drifted)
	res.DriftCount = len(drifted)
	res.DriftPaths = drifted
	return res, true
}

// generateDriftBaseline runs the platform adapter's Generate against a fresh
// isolated temp root, optionally seeded with representative user state. The
// temp root is removed before returning; the in-memory FileMapping content and
// checksums are all the drift comparison needs.
func generateDriftBaseline(platform string, cfg *config.HarnessConfig, seed func(string) error) (*adapter.PlatformFiles, bool) {
	tmp, err := os.MkdirTemp("", "autopus-drift-")
	if err != nil {
		return nil, false
	}
	defer os.RemoveAll(tmp)

	if seed != nil {
		if err := seed(tmp); err != nil {
			return nil, false
		}
	}

	ad := driftAdapterFor(platform, tmp)
	if ad == nil {
		return nil, false
	}
	pf, err := ad.Generate(context.Background(), cfg)
	if err != nil || pf == nil {
		return nil, false
	}
	return pf, true
}

// deterministicOverwriteSubset returns the OverwriteAlways target paths whose
// generated content is identical across the two seeded roots, mapped to their
// checksum. Marker and merge files are excluded by policy; state- and path-
// dependent OverwriteAlways files (e.g. statusline-user-command.txt) are
// excluded because their bytes differ between the empty and seeded roots.
func deterministicOverwriteSubset(a, b *adapter.PlatformFiles) map[string]string {
	bChecksum := make(map[string]string, len(b.Files))
	for _, f := range b.Files {
		if f.OverwritePolicy == adapter.OverwriteAlways {
			bChecksum[f.TargetPath] = f.Checksum
		}
	}

	out := make(map[string]string)
	for _, fa := range a.Files {
		if fa.OverwritePolicy != adapter.OverwriteAlways {
			continue
		}
		cb, ok := bChecksum[fa.TargetPath]
		if !ok {
			continue
		}
		if fa.Checksum != cb {
			continue
		}
		out[fa.TargetPath] = fa.Checksum
	}
	return out
}

// seedUserState writes representative pre-existing user state into a temp root
// so the determinism gate can detect state-dependent OverwriteAlways files. The
// merge-mode statusline plus a user command file makes the generated
// statusline-user-command.txt a function of prior state, and a user-authored
// CLAUDE.md body exercises marker-merge state.
func seedUserState(root string) error {
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return err
	}
	settings := `{"statusLine":{"command":".claude/statusline-combined.sh"}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "statusline-user-command.txt"), []byte("echo drift-seed\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# user preface\n"), 0o644)
}
