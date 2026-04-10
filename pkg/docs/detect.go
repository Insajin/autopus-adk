package docs

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
)

// versionSpecRe strips version specifiers from Python dependency strings.
var versionSpecRe = regexp.MustCompile(`[><=~!][^,\]]*`)

// extrasRe strips extras notation (e.g., [all]) from Python package names.
var extrasRe = regexp.MustCompile(`\[.*?\]`)

// wordBoundaryRe splits text into tokens by whitespace and common punctuation.
var wordBoundaryRe = regexp.MustCompile(`[\s,.:;()\[\]{}"']+`)

// DetectFromGoMod parses a go.mod file and returns each direct dependency module path
// as-is (e.g., "github.com/spf13/cobra").
// Indirect dependencies (lines with "// indirect") are skipped.
func DetectFromGoMod(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var libs []string
	seen := map[string]bool{}
	inRequire := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			break
		}
		if !inRequire {
			continue
		}
		if strings.Contains(line, "// indirect") {
			continue
		}
		// Format: <module-path> <version>
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		modPath := fields[0]
		if !seen[modPath] {
			seen[modPath] = true
			libs = append(libs, modPath)
		}
	}
	return libs, scanner.Err()
}

// packageJSON is used to unmarshal the dependency fields of a package.json file.
type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// DetectFromPackageJSON parses a package.json file and returns all dependency and
// devDependency package names.
func DetectFromPackageJSON(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var libs []string
	for name := range pkg.Dependencies {
		if !seen[name] {
			seen[name] = true
			libs = append(libs, name)
		}
	}
	for name := range pkg.DevDependencies {
		if !seen[name] {
			seen[name] = true
			libs = append(libs, name)
		}
	}
	return libs, nil
}

// DetectFromPyProjectToml parses a pyproject.toml file and returns dependency names,
// stripping version specifiers and extras (e.g., "fastapi[all]>=0.100.0" → "fastapi").
func DetectFromPyProjectToml(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var libs []string
	seen := map[string]bool{}
	inDeps := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "dependencies") && strings.Contains(line, "[") {
			inDeps = true
			continue
		}
		if inDeps {
			if line == "]" {
				break
			}
			// Strip surrounding quotes and trailing comma
			name := strings.Trim(line, `"',`)
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			// Strip extras like [all]
			name = extrasRe.ReplaceAllString(name, "")
			// Strip version specifiers like >=2.28.0
			name = versionSpecRe.ReplaceAllString(name, "")
			name = strings.TrimSpace(name)
			if name != "" && !seen[name] {
				seen[name] = true
				libs = append(libs, name)
			}
		}
	}
	return libs, scanner.Err()
}

// FilterStdLib removes standard library module names from the input slice for the
// given language ("go", "node", "python").
func FilterStdLib(lang string, libs []string) []string {
	var stdLib map[string]bool
	switch lang {
	case "go":
		stdLib = goStdLib
	case "node":
		stdLib = nodeStdLib
	case "python":
		stdLib = pythonStdLib
	default:
		return libs
	}

	result := make([]string, 0, len(libs))
	for _, lib := range libs {
		if !stdLib[lib] {
			result = append(result, lib)
		}
	}
	return result
}

// DetectFromText scans free-form text (e.g., SPEC or plan.md) and returns known
// library names for the given language, excluding standard library modules.
func DetectFromText(lang string, text string) []string {
	var knownLibs map[string]bool
	if lang == "go" {
		knownLibs = goKnownLibs
	} else {
		return nil
	}

	tokens := wordBoundaryRe.Split(text, -1)
	seen := map[string]bool{}
	var libs []string
	for _, tok := range tokens {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if knownLibs[tok] && !seen[tok] {
			seen[tok] = true
			libs = append(libs, tok)
		}
	}
	return FilterStdLib(lang, libs)
}
