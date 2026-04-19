package setup

import (
	"path/filepath"
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
		"ruff.toml": "Ruff",
		".flake8":   "Flake8",
		"setup.cfg": "Flake8",
		"tox.ini":   "Flake8",
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
