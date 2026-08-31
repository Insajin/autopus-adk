package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/evidence"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

// Well-known QAMESH locations and the default report filename.
const (
	RunsRelDir        = ".autopus/qa/runs"
	ReleasesRelDir    = ".autopus/qa/releases"
	RunIndexFile      = "run-index.json"
	ReleaseIndexFile  = "release-index.json"
	DefaultReportFile = "report.html"
)

// maxIndexBytes bounds index reads. Run and release indexes are summaries; a
// larger file means a corrupted or hostile input, not a bigger test suite.
const maxIndexBytes = 8 << 20

// Options controls report ingestion and projection.
type Options struct {
	ProjectDir       string
	RunIndexPath     string
	ReleaseIndexPath string
	Title            string
	// EmbedMedia inlines raw local screenshots from the capture directory. It is
	// opt-in because those bytes are local-only evidence: enabling it turns the
	// report from a shareable summary into local-only evidence itself.
	EmbedMedia bool
	// Now is injectable so report projections are deterministic under test.
	Now func() time.Time
}

type ingestedManifest struct {
	manifest evidence.Manifest
	ref      string
	// path is the absolute manifest path. Local capture media is addressed
	// relative to a directory derived from it, so the projection needs the real
	// location and not just the display ref.
	path string
}

type ingestion struct {
	projectRoot      string
	runIndexPath     string
	releaseIndexPath string
	runIndex         *qarun.Index
	releaseIndex     *qarelease.Index
	manifests        []ingestedManifest
	rejections       []Rejection
	// trusted is true only when the run index carried a supported schema version
	// and a passing redaction status. Every view derived from index content is
	// gated on it, so an untrusted index contributes identity fields and the
	// rejection banner, never evidence.
	trusted bool
}

// indexRefDisplay renders a ref recorded inside an index. Those refs are
// project-root relative by convention, so they are confined and shown as-is
// rather than resolved against the process working directory.
func indexRefDisplay(root, ref string) string {
	if strings.TrimSpace(ref) == "" {
		return ""
	}
	if rel, reason := confinedRef(root, ref); reason == "" {
		return rel
	}
	return evidence.RedactText(filepath.ToSlash(ref))
}

func (i *ingestion) reject(ref, reason string) {
	i.rejections = append(i.rejections, Rejection{Ref: evidence.RedactText(ref), Reason: reason})
}

// ResolveLatestIndex returns the most recently modified index file under
// <projectDir>/<relDir>/*/<filename>, or "" when none exists.
func ResolveLatestIndex(projectDir, relDir, filename string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(projectDir, filepath.FromSlash(relDir), "*", filename))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	sort.Slice(matches, func(a, b int) bool {
		return indexModTime(matches[a]).After(indexModTime(matches[b]))
	})
	return matches[0], nil
}

func indexModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// load ingests the run index, its evidence manifests, and the optional release
// index. Every refusal is recorded as a Rejection instead of being dropped, so
// the rendered report always states what it could not read.
func load(opts Options) (ingestion, error) {
	root, err := realPath(opts.ProjectDir)
	if err != nil {
		return ingestion{}, fmt.Errorf("resolve project dir: %w", err)
	}
	in := ingestion{projectRoot: root}
	if err := loadRunIndex(&in, opts); err != nil {
		return ingestion{}, err
	}
	loadReleaseIndex(&in, opts)
	return in, nil
}

func loadRunIndex(in *ingestion, opts Options) error {
	path, err := resolveIndexPath(opts.ProjectDir, opts.RunIndexPath, RunsRelDir, RunIndexFile)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("no QAMESH run index found under %s; run `auto qa run` first", RunsRelDir)
	}
	in.runIndexPath = path
	var index qarun.Index
	if err := readIndexJSON(path, &index); err != nil {
		return fmt.Errorf("read run index: %w", err)
	}
	if index.SchemaVersion != qarun.RunIndexSchemaVersion {
		in.reject(path, fmt.Sprintf("unsupported_schema_version:%s", index.SchemaVersion))
		return nil
	}
	in.runIndex = &index
	if index.RedactionStatus.Status != "passed" {
		in.reject(path, fmt.Sprintf("redaction_not_passed:%s", index.RedactionStatus.Status))
		return nil
	}
	in.trusted = true
	loadManifests(in, index.ManifestPaths)
	return nil
}

func loadReleaseIndex(in *ingestion, opts Options) {
	path, err := resolveIndexPath(opts.ProjectDir, opts.ReleaseIndexPath, ReleasesRelDir, ReleaseIndexFile)
	if err != nil || path == "" {
		return
	}
	in.releaseIndexPath = path
	var index qarelease.Index
	if err := readIndexJSON(path, &index); err != nil {
		in.reject(path, "unreadable_release_index")
		return
	}
	if index.SchemaVersion != qarelease.IndexSchemaVersion {
		in.reject(path, fmt.Sprintf("unsupported_schema_version:%s", index.SchemaVersion))
		return
	}
	if index.RedactionStatus == qarelease.RedactionBlocked {
		in.reject(path, "redaction_blocked")
		return
	}
	in.releaseIndex = &index
}

// loadManifests applies the evidence publication boundary to every manifest ref:
// project-root confinement, duplicate-key rejection, schema validation, and
// artifact path confinement. A manifest that fails any gate is rejected whole.
func loadManifests(in *ingestion, refs []string) {
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		rel, reason := confinedRef(in.projectRoot, ref)
		if reason != "" {
			in.reject(ref, reason)
			continue
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		abs := filepath.Join(in.projectRoot, filepath.FromSlash(rel))
		manifest, err := evidence.LoadManifest(abs)
		if err != nil {
			in.reject(rel, "unreadable_manifest")
			continue
		}
		if err := manifest.Validate(); err != nil {
			in.reject(rel, "invalid_manifest:"+firstLine(err.Error()))
			continue
		}
		resolved, err := evidence.ResolveArtifactPaths(manifest, filepath.Dir(abs))
		if err != nil {
			in.reject(rel, "unsafe_artifact_path")
			continue
		}
		in.manifests = append(in.manifests, ingestedManifest{manifest: resolved, ref: rel, path: abs})
	}
}

func resolveIndexPath(projectDir, explicit, relDir, filename string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	return ResolveLatestIndex(projectDir, relDir, filename)
}

func readIndexJSON(path string, target any) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > maxIndexBytes {
		return fmt.Errorf("index exceeds %d bytes", maxIndexBytes)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

// confinedRef normalizes an index-recorded manifest ref to a project-relative
// slash path. Absolute refs are accepted only when they resolve inside the
// project root, and are rewritten to relative so no local user path is rendered.
func confinedRef(root, ref string) (string, string) {
	text := strings.TrimSpace(ref)
	if text == "" {
		return "", "invalid_ref:empty"
	}
	if strings.ContainsAny(text, "\x00\r\n\t") {
		return "", "invalid_ref:control_character"
	}
	clean := filepath.Clean(filepath.FromSlash(text))
	if filepath.IsAbs(clean) {
		resolved, err := realPath(clean)
		if err != nil {
			return "", "unreadable_manifest"
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || escapesRoot(rel) {
			return "", "unsafe_ref:absolute_outside_project"
		}
		return filepath.ToSlash(rel), ""
	}
	if clean == "." || escapesRoot(clean) {
		return "", "unsafe_ref:path_traversal"
	}
	return filepath.ToSlash(clean), ""
}

func escapesRoot(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func realPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
}

func firstLine(value string) string {
	if idx := strings.IndexAny(value, "\r\n"); idx >= 0 {
		return value[:idx]
	}
	return value
}
