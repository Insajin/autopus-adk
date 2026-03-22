package content

import (
	"os"
	"path/filepath"
	"strings"
)

// ActivationContext holds the context derived from the user's query and active files.
// It is used by the activator to select relevant skills and agents.
type ActivationContext struct {
	// UserQuery is the raw text request from the user.
	UserQuery string
	// ActiveFiles is the list of files currently being edited or referenced.
	ActiveFiles []string
	// FileExtensions contains deduplicated file extensions present in ActiveFiles.
	FileExtensions []string
	// ProjectMarkers lists detected project root markers (e.g., go.mod, package.json).
	ProjectMarkers []string
	// Language is the inferred primary programming language for the session.
	Language string
}

// knownMarkers is the set of project root marker filenames to probe.
var knownMarkers = []string{
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"pom.xml",
}

// DetectContext builds an ActivationContext from the user query and the active file list.
// It probes the filesystem for project markers using the directories of the active files
// as well as the current working directory.
func DetectContext(query string, files []string) ActivationContext {
	exts := extractExtensions(files)
	markers := detectMarkers(files)
	lang := detectLanguage(exts, markers)

	return ActivationContext{
		UserQuery:      query,
		ActiveFiles:    files,
		FileExtensions: exts,
		ProjectMarkers: markers,
		Language:       lang,
	}
}

// extractExtensions returns a deduplicated slice of file extensions from the given paths.
// Extensions are returned in lower-case with the leading dot (e.g., ".go").
func extractExtensions(files []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == "" {
			continue
		}
		if _, ok := seen[ext]; !ok {
			seen[ext] = struct{}{}
			result = append(result, ext)
		}
	}
	return result
}

// detectMarkers probes for known project marker files. It searches the directories
// of each active file as well as the current working directory.
func detectMarkers(files []string) []string {
	dirs := collectSearchDirs(files)

	seen := make(map[string]struct{})
	var result []string
	for _, dir := range dirs {
		for _, marker := range knownMarkers {
			candidate := filepath.Join(dir, marker)
			if _, err := os.Stat(candidate); err == nil {
				if _, ok := seen[marker]; !ok {
					seen[marker] = struct{}{}
					result = append(result, marker)
				}
			}
		}
	}
	return result
}

// collectSearchDirs returns a deduplicated list of directories to probe.
// It includes the directories of all active files plus the current working directory.
func collectSearchDirs(files []string) []string {
	seen := make(map[string]struct{})
	var dirs []string

	addDir := func(d string) {
		if d == "" {
			return
		}
		if _, ok := seen[d]; !ok {
			seen[d] = struct{}{}
			dirs = append(dirs, d)
		}
	}

	for _, f := range files {
		addDir(filepath.Dir(f))
	}

	if cwd, err := os.Getwd(); err == nil {
		addDir(cwd)
	}

	return dirs
}

// markerLanguage maps a project marker filename to the primary language it implies.
var markerLanguage = map[string]string{
	"go.mod":        "go",
	"package.json":  "javascript",
	"Cargo.toml":    "rust",
	"pyproject.toml": "python",
	"pom.xml":       "java",
}

// extLanguage maps a lower-case file extension (with dot) to a language name.
var extLanguage = map[string]string{
	".go":   "go",
	".py":   "python",
	".js":   "javascript",
	".ts":   "javascript",
	".rs":   "rust",
	".java": "java",
}

// detectLanguage infers the primary language from file extensions and project markers.
// Markers take precedence over extensions. Returns an empty string when undetermined.
func detectLanguage(extensions []string, markers []string) string {
	// Markers have higher confidence — return the first recognisable one.
	for _, m := range markers {
		if lang, ok := markerLanguage[m]; ok {
			return lang
		}
	}

	// Fall back to the first recognisable extension.
	for _, ext := range extensions {
		if lang, ok := extLanguage[ext]; ok {
			return lang
		}
	}

	return ""
}
