package adapter_test

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// generatedRefRe matches the local path references that generated harness
// surfaces make. Two families matter:
//
//   - installed harness surfaces (.claude/, .codex/, .gemini/, .opencode/,
//     .omp/, .agents/) plus the root documents, which a consumer repo really
//     has after `auto init`;
//   - ADK source directories (content/, templates/), which no install manifest
//     ever writes. A generated file naming one of those only resolves inside
//     this dogfood repo, where the ADK checkout happens to sit next to the
//     installed surface.
//
// Group 1 is the preceding boundary, group 2 an external prefix that takes the
// reference out of scope, group 3 the reference itself. .autopus/ is
// deliberately absent from the surfaces: it is runtime state written by
// commands, not installed content, so a manifest cannot answer for it.
var generatedRefRe = regexp.MustCompile(
	`(?m)(^|[^A-Za-z0-9_./~-])(~/|autopus-adk/)?(` +
		`\.(?:claude|codex|gemini|opencode|omp|agents)/[A-Za-z0-9_.*<>{}/-]*` +
		`|(?:pkg/|internal/|cmd/)?(?:content|templates)/[A-Za-z0-9_.*<>{}/-]*` +
		`|(?:AGENTS|CLAUDE|GEMINI)\.md(?:#[a-z0-9-]+)?[A-Za-z0-9_./-]*` +
		`)`)

// headingRe matches a markdown ATX heading so an anchored reference can be
// checked against the sections its target file actually declares.
var headingRe = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*$`)

// wildcardChars mark a reference that names a family of files rather than one.
const wildcardChars = "*<{"

// installedSurface is the union of every platform's generated target paths.
// The union is the right oracle because the reported defect appears in a
// workspace with all platforms installed, and a generated file for one
// platform legitimately points at another platform's surface.
type installedSurface struct {
	files map[string]string // slash path -> file body
	dirs  map[string]bool   // every ancestor directory, with trailing slash
}

func newInstalledSurface() *installedSurface {
	return &installedSurface{files: map[string]string{}, dirs: map[string]bool{}}
}

func (s *installedSurface) add(files []adapter.FileMapping) {
	for _, f := range files {
		p := path.Clean(strings.ReplaceAll(f.TargetPath, "\\", "/"))
		s.files[p] = string(f.Content)
		for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
			s.dirs[dir+"/"] = true
		}
	}
}

// slugify converts a markdown heading to its GitHub-style anchor.
func slugify(heading string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}

func (s *installedSurface) hasAnchor(file, anchor string) bool {
	body, ok := s.files[file]
	if !ok {
		return false
	}
	for _, m := range headingRe.FindAllStringSubmatch(body, -1) {
		if slugify(m[1]) == anchor {
			return true
		}
	}
	return false
}

// checkableRef reports whether raw is a reference an installer can answer for,
// returning it split into path and markdown anchor. Prose that merely contains
// a slash ("file content/JSON values") and bare directory examples in tree
// diagrams are not references, so a checkable one must name a file by
// extension, a family by wildcard, or a directory by trailing slash, and must
// carry at least one segment below its root.
func checkableRef(raw string) (ref, anchor string, ok bool) {
	ref = strings.TrimRight(raw, ".,;:)")
	if i := strings.Index(ref, "#"); i >= 0 {
		ref, anchor = ref[:i], ref[i+1:]
	}
	if ref == "" {
		return "", "", false
	}
	root, rest, hasRest := strings.Cut(ref, "/")
	if !hasRest {
		// A bare root document such as AGENTS.md.
		return ref, anchor, strings.Contains(root, ".")
	}
	if strings.TrimSuffix(rest, "/") == "" {
		return "", "", false
	}
	last := path.Base(ref)
	if strings.HasSuffix(ref, "/") || strings.ContainsAny(last, wildcardChars) {
		return ref, anchor, true
	}
	return ref, anchor, strings.Contains(last, ".")
}

// resolve reports whether a reference points at something installed. A
// reference into an ADK source directory never does, which is the whole point
// of scanning for it.
func (s *installedSurface) resolve(ref, anchor string) bool {
	if strings.Contains(ref, "content/") || strings.Contains(ref, "templates/") {
		return false
	}
	if anchor != "" {
		return s.hasAnchor(ref, anchor)
	}
	if strings.HasSuffix(ref, "/") {
		return s.dirs[ref]
	}
	if _, ok := s.files[ref]; ok {
		return true
	}
	// A pattern such as .codex/skills/codex-<name>/SKILL.md or
	// .codex/agents/*.toml names a family, so only its literal parent
	// directory can be checked.
	if i := strings.IndexAny(ref, wildcardChars); i >= 0 {
		parent := ref[:i]
		if j := strings.LastIndex(parent, "/"); j >= 0 {
			return s.dirs[parent[:j+1]]
		}
		return false
	}
	return s.dirs[ref+"/"]
}

// collectDangling records every unresolved reference in body under the file
// that made it, so a failure names both the bad path and where to fix it.
func (s *installedSurface) collectDangling(file, body string, into map[string]map[string]int) {
	for _, m := range generatedRefRe.FindAllStringSubmatch(body, -1) {
		if m[2] != "" {
			// ~/ is user-home state and autopus-adk/ explicitly names the ADK
			// repo; neither is this workspace's installed surface.
			continue
		}
		ref, anchor, ok := checkableRef(m[3])
		if !ok || s.resolve(ref, anchor) {
			continue
		}
		key := ref
		if anchor != "" {
			key += "#" + anchor
		}
		if into[key] == nil {
			into[key] = map[string]int{}
		}
		into[key][file]++
	}
}

func sortedRefs(m map[string]map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// refSummary renders occurrence count plus up to three example files.
func refSummary(sites map[string]int) (total int, examples string) {
	files := make([]string, 0, len(sites))
	for f, n := range sites {
		files = append(files, f)
		total += n
	}
	sort.Strings(files)
	if len(files) > 3 {
		files = append(files[:3:3], "...")
	}
	return total, strings.Join(files, ", ")
}
