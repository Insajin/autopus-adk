package setup

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const maxSampleFiles = 20

// AnalyzeConventions scans project source files to detect actual coding conventions.
func AnalyzeConventions(dir string, langs []Language) map[string]ConventionSample {
	conventions := make(map[string]ConventionSample)

	for _, lang := range langs {
		var sample ConventionSample
		switch lang.Name {
		case "Go":
			sample = analyzeGoConventions(dir)
		case "TypeScript":
			sample = analyzeTSConventions(dir)
		case "JavaScript":
			sample = analyzeJSConventions(dir)
		case "Python":
			sample = analyzePythonConventions(dir)
		case "Rust":
			sample = analyzeRustConventions(dir)
		default:
			continue
		}
		conventions[lang.Name] = sample
	}

	return conventions
}

func analyzeGoConventions(dir string) ConventionSample {
	sample := ConventionSample{}

	// Detect file naming pattern
	goFiles := collectSourceFiles(dir, ".go", maxSampleFiles)
	sample.FileNaming = detectFileNaming(goFiles)
	sample.ExampleFiles = pickExamples(goFiles, 3)

	// Detect error handling patterns
	sample.ErrorPatterns = sampleGoErrorPatterns(dir, goFiles)

	// Detect import style
	sample.ImportStyle = detectGoImportStyle(goFiles)

	// Detect linter
	linterConfigs := map[string]string{
		".golangci.yml":  "golangci-lint",
		".golangci.yaml": "golangci-lint",
		".golangci.toml": "golangci-lint",
	}
	for file, name := range linterConfigs {
		if fileExists(filepath.Join(dir, file)) {
			sample.HasLinter = true
			sample.LinterName = name
			break
		}
	}

	// Detect formatter (gofmt is built-in, check for goimports)
	sample.HasFormatter = true
	sample.FormatterName = "gofmt"

	return sample
}

func analyzeTSConventions(dir string) ConventionSample {
	sample := ConventionSample{}

	tsFiles := collectSourceFiles(dir, ".ts", maxSampleFiles)
	tsxFiles := collectSourceFiles(dir, ".tsx", maxSampleFiles)
	allFiles := append(tsFiles, tsxFiles...)
	sample.FileNaming = detectFileNaming(allFiles)
	sample.ExampleFiles = pickExamples(allFiles, 3)

	// Detect linter
	for _, f := range []string{".eslintrc.json", ".eslintrc.js", ".eslintrc.yml", ".eslintrc.yaml", "eslint.config.js", "eslint.config.mjs"} {
		if fileExists(filepath.Join(dir, f)) {
			sample.HasLinter = true
			sample.LinterName = "ESLint"
			break
		}
	}
	if !sample.HasLinter {
		if fileExists(filepath.Join(dir, "biome.json")) {
			sample.HasLinter = true
			sample.LinterName = "Biome"
		}
	}

	// Detect formatter
	if fileExists(filepath.Join(dir, ".prettierrc")) || fileExists(filepath.Join(dir, ".prettierrc.json")) ||
		fileExists(filepath.Join(dir, ".prettierrc.js")) || fileExists(filepath.Join(dir, "prettier.config.js")) {
		sample.HasFormatter = true
		sample.FormatterName = "Prettier"
	}
	if fileExists(filepath.Join(dir, "biome.json")) {
		sample.HasFormatter = true
		sample.FormatterName = "Biome"
	}

	return sample
}

func analyzeJSConventions(dir string) ConventionSample {
	// Reuse TS logic since conventions are similar
	sample := analyzeTSConventions(dir)
	jsFiles := collectSourceFiles(dir, ".js", maxSampleFiles)
	if len(jsFiles) > 0 {
		sample.FileNaming = detectFileNaming(jsFiles)
		sample.ExampleFiles = pickExamples(jsFiles, 3)
	}
	return sample
}

func analyzePythonConventions(dir string) ConventionSample {
	sample := ConventionSample{}

	pyFiles := collectSourceFiles(dir, ".py", maxSampleFiles)
	sample.FileNaming = detectFileNaming(pyFiles)
	sample.ExampleFiles = pickExamples(pyFiles, 3)

	// Detect linter
	linters := map[string]string{
		"ruff.toml":      "Ruff",
		".flake8":        "Flake8",
		"setup.cfg":      "Flake8",
		"tox.ini":        "Flake8",
	}
	for file, name := range linters {
		if fileExists(filepath.Join(dir, file)) {
			sample.HasLinter = true
			sample.LinterName = name
			break
		}
	}
	// Check pyproject.toml for ruff/flake8 config
	if !sample.HasLinter {
		if hasTomlSection(filepath.Join(dir, "pyproject.toml"), "[tool.ruff") {
			sample.HasLinter = true
			sample.LinterName = "Ruff"
		} else if hasTomlSection(filepath.Join(dir, "pyproject.toml"), "[tool.flake8") {
			sample.HasLinter = true
			sample.LinterName = "Flake8"
		}
	}

	// Detect formatter
	if fileExists(filepath.Join(dir, "pyproject.toml")) {
		if hasTomlSection(filepath.Join(dir, "pyproject.toml"), "[tool.black") {
			sample.HasFormatter = true
			sample.FormatterName = "Black"
		} else if hasTomlSection(filepath.Join(dir, "pyproject.toml"), "[tool.ruff.format") {
			sample.HasFormatter = true
			sample.FormatterName = "Ruff"
		}
	}

	return sample
}

func analyzeRustConventions(dir string) ConventionSample {
	sample := ConventionSample{}

	rsFiles := collectSourceFiles(dir, ".rs", maxSampleFiles)
	sample.FileNaming = detectFileNaming(rsFiles)
	sample.ExampleFiles = pickExamples(rsFiles, 3)

	// Rust has built-in tools
	sample.HasLinter = true
	sample.LinterName = "clippy"
	sample.HasFormatter = true
	sample.FormatterName = "rustfmt"

	// Check for rustfmt config
	if fileExists(filepath.Join(dir, "rustfmt.toml")) || fileExists(filepath.Join(dir, ".rustfmt.toml")) {
		sample.FormatterName = "rustfmt (custom config)"
	}

	return sample
}

// collectSourceFiles gathers source files with the given extension, up to maxCount.
func collectSourceFiles(dir, ext string, maxCount int) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and vendored code
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ext) && !strings.Contains(path, "vendor/") {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, rel)
			if len(files) >= maxCount {
				return filepath.SkipAll
			}
		}
		return nil
	})
	return files
}

// detectFileNaming analyzes file names to determine the dominant naming convention.
func detectFileNaming(files []string) string {
	counts := map[string]int{
		"snake_case":  0,
		"kebab-case":  0,
		"camelCase":   0,
		"PascalCase":  0,
	}

	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		// Skip test files and single-word names (ambiguous)
		if strings.Contains(name, "_test") || strings.Contains(name, ".test") ||
			strings.Contains(name, ".spec") {
			continue
		}
		if !strings.ContainsAny(name, "_-") && !hasUpperCase(name[1:]) {
			continue // Single word, can't determine pattern
		}

		if strings.Contains(name, "_") {
			counts["snake_case"]++
		} else if strings.Contains(name, "-") {
			counts["kebab-case"]++
		} else if len(name) > 0 && unicode.IsUpper(rune(name[0])) {
			counts["PascalCase"]++
		} else if hasUpperCase(name) {
			counts["camelCase"]++
		}
	}

	// Find dominant pattern
	maxCount := 0
	result := "snake_case" // default
	for pattern, count := range counts {
		if count > maxCount {
			maxCount = count
			result = pattern
		}
	}
	return result
}

func hasUpperCase(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

// sampleGoErrorPatterns extracts unique error handling patterns from Go files.
func sampleGoErrorPatterns(dir string, files []string) []string {
	patterns := make(map[string]int)
	errReturnRe := regexp.MustCompile(`return\s+.*fmt\.Errorf\((.+)\)`)
	errWrapRe := regexp.MustCompile(`return\s+.*errors\.(New|Wrap|Wrapf)\(`)

	for _, f := range files {
		if len(patterns) >= 5 {
			break
		}
		path := filepath.Join(dir, f)
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if errReturnRe.MatchString(line) {
				// Detect wrapping style
				if strings.Contains(line, "%w") {
					patterns["fmt.Errorf with %w wrapping"]++
				} else {
					patterns["fmt.Errorf without wrapping"]++
				}
			}
			if errWrapRe.MatchString(line) {
				patterns["errors.Wrap (pkg/errors style)"]++
			}
			if strings.Contains(line, "if err != nil {") {
				patterns["if err != nil guard"]++
			}
		}
		_ = file.Close()
	}

	var result []string
	for pattern := range patterns {
		result = append(result, pattern)
	}
	return result
}

// detectGoImportStyle checks if imports are grouped (stdlib, internal, external).
func detectGoImportStyle(files []string) string {
	// Check first few files for import grouping
	for _, f := range files {
		if len(f) > 0 {
			// Simple heuristic: if import blocks have blank lines, they're grouped
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			content := string(data)
			if strings.Contains(content, "import (") {
				// Count blank lines within import blocks
				inImport := false
				hasBlankLine := false
				for _, line := range strings.Split(content, "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed == "import (" {
						inImport = true
						continue
					}
					if inImport && trimmed == ")" {
						break
					}
					if inImport && trimmed == "" {
						hasBlankLine = true
					}
				}
				if hasBlankLine {
					return "grouped (stdlib / internal / external)"
				}
				return "ungrouped"
			}
		}
	}
	return "unknown"
}

func pickExamples(files []string, n int) []string {
	if len(files) <= n {
		return files
	}
	return files[:n]
}

func hasTomlSection(path, section string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), section)
}
