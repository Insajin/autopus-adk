package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// specIDValidPattern constrains --spec input to a safe SPEC identifier so it can
// never resolve outside the workspace .autopus/specs tree.
var specIDValidPattern = regexp.MustCompile(`^SPEC-[A-Z0-9-]+$`)

// specIDInPath extracts the SPEC id from a repo-relative dirty path.
var specIDInPath = regexp.MustCompile(`^\.autopus/specs/(SPEC-[A-Z0-9-]+)/`)

// codePrefixes are the module-detection roots from the doc-storage rule.
var codePrefixes = []string{"pkg/", "cmd/", "internal/", "src/", "app/"}

// ownedPathPattern captures file-path-like tokens from SPEC markdown for the
// --spec ownership split.
var ownedPathPattern = regexp.MustCompile(`[A-Za-z0-9_./-]+\.(?:go|ts|tsx|js|jsx|py|rs|md|json|yaml|yml|toml)`)

// validateSpecID rejects any --spec value that is not a bare SPEC identifier,
// blocking path traversal before any filesystem access.
func validateSpecID(id string) error {
	if !specIDValidPattern.MatchString(id) {
		return fmt.Errorf("invalid --spec %q: must match ^SPEC-[A-Z0-9-]+$", id)
	}
	return nil
}

// specIDFromPath returns the SPEC id owning a dirty path, if any.
func specIDFromPath(rel string) (string, bool) {
	m := specIDInPath.FindStringSubmatch(rel)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// readSpecReferences concatenates spec.md and plan.md for path-token extraction.
// SPEC markdown is untrusted local input: only path tokens are read, not executed.
func readSpecReferences(absSpecDir string) string {
	var sb strings.Builder
	for _, name := range []string{"spec.md", "plan.md"} {
		data, err := os.ReadFile(filepath.Join(absSpecDir, name))
		if err != nil {
			continue
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// referencedModules returns the sorted set of known modules whose code paths are
// referenced (as "<module>/<codeprefix>...") inside the SPEC text.
func referencedModules(text string, modules map[string]bool) []string {
	found := map[string]bool{}
	for m := range modules {
		for _, p := range codePrefixes {
			if strings.Contains(text, m+"/"+p) {
				found[m] = true
				break
			}
		}
	}
	var out []string
	for m := range found {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// classifySpecLocation compares a SPEC's current repo against the module derived
// from its referenced code paths and returns a warning when they diverge. An
// empty owned set is ambiguous and suppressed to avoid false positives.
func classifySpecLocation(specID, repoPath string, owned []string) string {
	if len(owned) == 0 {
		return ""
	}
	if len(owned) == 1 {
		m := owned[0]
		switch {
		case repoPath == ".":
			return fmt.Sprintf(
				"WARN  misplacement: SPEC %s at root (.) references only module %s paths -> expected %s/.autopus/specs/",
				specID, m, m)
		case repoPath != m:
			return fmt.Sprintf(
				"WARN  location-mismatch: SPEC %s at %s references only module %s paths -> expected %s/.autopus/specs/",
				specID, repoPath, m, m)
		default:
			return ""
		}
	}
	// 2+ modules referenced -> cross-module ownership belongs at the meta root.
	if repoPath != "." {
		return fmt.Sprintf(
			"WARN  location-mismatch: SPEC %s at %s references cross-module paths -> detected owner cross-module, expected .autopus/specs/ (root)",
			specID, repoPath)
	}
	return ""
}

// detectSpecViolations checks every dirty SPEC directory for a location that
// disagrees with its referenced module ownership.
func detectSpecViolations(repos []repoDirty, modules map[string]bool) []string {
	type specRef struct {
		repoPath string
		specID   string
		absDir   string
	}
	seen := map[string]bool{}
	var refs []specRef
	for _, r := range repos {
		for _, f := range r.Files {
			id, ok := specIDFromPath(f.Rel)
			if !ok {
				continue
			}
			key := r.Path + "|" + id
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, specRef{
				repoPath: r.Path,
				specID:   id,
				absDir:   filepath.Join(r.AbsPath, ".autopus", "specs", id),
			})
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].specID != refs[j].specID {
			return refs[i].specID < refs[j].specID
		}
		return refs[i].repoPath < refs[j].repoPath
	})

	var warnings []string
	for _, ref := range refs {
		owned := referencedModules(readSpecReferences(ref.absDir), modules)
		if w := classifySpecLocation(ref.specID, ref.repoPath, owned); w != "" {
			warnings = append(warnings, w)
		}
	}
	return warnings
}

// splitSpecOwnership locates the SPEC directory across repos and partitions the
// hosting repo's dirty files into files the SPEC owns and unrelated files.
func splitSpecOwnership(repos []repoDirty, specID string) (owned, unrelated []string, found bool) {
	var host *repoDirty
	for i := range repos {
		dir := filepath.Join(repos[i].AbsPath, ".autopus", "specs", specID)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			host = &repos[i]
			break
		}
	}
	if host == nil {
		return nil, nil, false
	}

	absSpecDir := filepath.Join(host.AbsPath, ".autopus", "specs", specID)
	ownedTokens := ownedPathPattern.FindAllString(readSpecReferences(absSpecDir), -1)
	for _, f := range host.Files {
		if isOwnedPath(f.Rel, ownedTokens, host.Path, specID) {
			owned = append(owned, f.Rel)
		} else {
			unrelated = append(unrelated, f.Rel)
		}
	}
	sort.Strings(owned)
	sort.Strings(unrelated)
	return owned, unrelated, true
}

// isOwnedPath reports whether a dirty file belongs to the target SPEC, matching
// either the SPEC's own directory or a path token declared in its markdown.
func isOwnedPath(rel string, ownedTokens []string, hostPath, specID string) bool {
	if strings.HasPrefix(rel, ".autopus/specs/"+specID+"/") {
		return true
	}
	for _, tok := range ownedTokens {
		if tok == rel {
			return true
		}
		if hostPath != "." && strings.HasPrefix(tok, hostPath+"/") && tok[len(hostPath)+1:] == rel {
			return true
		}
	}
	return false
}
