package cli

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

const manifestSuffix = "-manifest.json"

// legacyManifestAliases maps a superseded platform token to the current token
// that replaced it. Such manifests are still orphans (their platform is not
// configured), but the removal hint names the successor for the operator.
var legacyManifestAliases = map[string]string{
	"gemini-cli": "antigravity-cli",
}

// orphanManifestResult is the orphan-manifest verdict. Present reports whether
// any .autopus/*-manifest.json exists at all — when false the check is skipped
// entirely (REQ-008). Paths lists the orphan manifest paths (repo-relative,
// slash form, sorted); Aliases maps each orphan token to its successor when the
// orphan is a known legacy alias.
type orphanManifestResult struct {
	Present bool
	Paths   []string
	Aliases map[string]string
}

// detectOrphanManifests reports .autopus/<platform>-manifest.json files whose
// platform token is not in the configured platform set (REQ-004). It reads only
// file names from the .autopus directory — never file contents — so a tampered
// manifest body cannot influence the result, and path.Base strips any traversal.
func detectOrphanManifests(dir string, cfg *config.HarnessConfig) orphanManifestResult {
	res := orphanManifestResult{Aliases: map[string]string{}}
	if cfg == nil {
		return res
	}

	entries, err := os.ReadDir(filepath.Join(dir, ".autopus"))
	if err != nil {
		// No .autopus directory (or unreadable) means nothing to inspect.
		return res
	}

	configured := make(map[string]bool, len(cfg.Platforms))
	for _, p := range cfg.Platforms {
		configured[p] = true
	}

	var orphans []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		base := path.Base(entry.Name())
		if !strings.HasSuffix(base, manifestSuffix) {
			continue
		}
		res.Present = true

		token := strings.TrimSuffix(base, manifestSuffix)
		if token == "" || configured[token] {
			continue
		}
		orphans = append(orphans, ".autopus/"+base)
		if successor, ok := legacyManifestAliases[token]; ok {
			res.Aliases[token] = successor
		}
	}

	sort.Strings(orphans)
	res.Paths = orphans
	return res
}
