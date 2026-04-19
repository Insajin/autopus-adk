package setup

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const maxDepth = 3

// Scan analyzes a project directory and returns ProjectInfo.
func Scan(projectDir string) (*ProjectInfo, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}

	info := &ProjectInfo{
		Name:    filepath.Base(absDir),
		RootDir: absDir,
	}

	info.Languages = detectLanguages(absDir)
	info.Frameworks = detectFrameworks(absDir)
	info.BuildFiles = detectBuildFiles(absDir)
	info.EntryPoints = detectEntryPoints(absDir, info.Languages)
	info.TestConfig = detectTestConfig(absDir, info.Languages, info.BuildFiles)
	info.Structure = scanDirectoryTree(absDir, 0)
	info.Conventions = AnalyzeConventions(absDir, info.Languages)
	info.Workspaces = DetectWorkspaces(absDir)

	return info, nil
}

func detectLanguages(dir string) []Language {
	var langs []Language

	// Go
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		ver := ""
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "go ") {
				ver = strings.TrimPrefix(line, "go ")
				break
			}
		}
		langs = append(langs, Language{
			Name:       "Go",
			Version:    ver,
			BuildFiles: []string{"go.mod"},
		})
	}

	// TypeScript / JavaScript
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pkg struct {
			DevDeps map[string]string `json:"devDependencies"`
			Deps    map[string]string `json:"dependencies"`
		}
		_ = json.Unmarshal(data, &pkg)

		if _, ok := pkg.DevDeps["typescript"]; ok {
			langs = append(langs, Language{
				Name:       "TypeScript",
				Version:    pkg.DevDeps["typescript"],
				BuildFiles: []string{"package.json", "tsconfig.json"},
			})
		} else {
			langs = append(langs, Language{
				Name:       "JavaScript",
				BuildFiles: []string{"package.json"},
			})
		}
	}

	// Python
	if fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "setup.py")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) {

		bf := []string{}
		for _, f := range []string{"pyproject.toml", "setup.py", "requirements.txt"} {
			if fileExists(filepath.Join(dir, f)) {
				bf = append(bf, f)
			}
		}
		langs = append(langs, Language{
			Name:       "Python",
			BuildFiles: bf,
		})
	}

	// Rust
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		langs = append(langs, Language{
			Name:       "Rust",
			BuildFiles: []string{"Cargo.toml"},
		})
	}

	return langs
}

func detectFrameworks(dir string) []Framework {
	var frameworks []Framework

	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pkg struct {
			Deps    map[string]string `json:"dependencies"`
			DevDeps map[string]string `json:"devDependencies"`
		}
		_ = json.Unmarshal(data, &pkg)

		knownFrameworks := map[string]string{
			"react":   "React",
			"vue":     "Vue",
			"next":    "Next.js",
			"express": "Express",
			"nestjs":  "NestJS",
			"angular": "Angular",
		}
		allDeps := mergeMaps(pkg.Deps, pkg.DevDeps)
		for key, name := range knownFrameworks {
			if ver, ok := allDeps[key]; ok {
				frameworks = append(frameworks, Framework{Name: name, Version: ver})
			}
		}
	}

	return frameworks
}

func detectBuildFiles(dir string) []BuildFile {
	var buildFiles []BuildFile

	// Makefile
	if fileExists(filepath.Join(dir, "Makefile")) {
		bf := BuildFile{
			Path:     "Makefile",
			Type:     "makefile",
			Commands: parseMakefileTargets(filepath.Join(dir, "Makefile")),
		}
		buildFiles = append(buildFiles, bf)
	}

	// package.json scripts
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		_ = json.Unmarshal(data, &pkg)
		if len(pkg.Scripts) > 0 {
			buildFiles = append(buildFiles, BuildFile{
				Path:     "package.json",
				Type:     "package.json",
				Commands: pkg.Scripts,
			})
		}
	}

	// go.mod (standard go commands)
	if fileExists(filepath.Join(dir, "go.mod")) {
		buildFiles = append(buildFiles, BuildFile{
			Path: "go.mod",
			Type: "go.mod",
			Commands: map[string]string{
				"build": "go build ./...",
				"test":  "go test ./...",
				"vet":   "go vet ./...",
			},
		})
	}

	// pyproject.toml
	if fileExists(filepath.Join(dir, "pyproject.toml")) {
		cmds := parsePyprojectScripts(filepath.Join(dir, "pyproject.toml"))
		if len(cmds) > 0 {
			buildFiles = append(buildFiles, BuildFile{
				Path:     "pyproject.toml",
				Type:     "pyproject.toml",
				Commands: cmds,
			})
		}
	}

	// Cargo.toml
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		buildFiles = append(buildFiles, BuildFile{
			Path: "Cargo.toml",
			Type: "cargo.toml",
			Commands: map[string]string{
				"build":  "cargo build",
				"test":   "cargo test",
				"check":  "cargo check",
				"clippy": "cargo clippy",
			},
		})
	}

	// docker-compose.yml
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if fileExists(filepath.Join(dir, name)) {
			buildFiles = append(buildFiles, BuildFile{
				Path: name,
				Type: "docker-compose",
				Commands: map[string]string{
					"up":   "docker compose up -d",
					"down": "docker compose down",
					"logs": "docker compose logs -f",
				},
			})
			break
		}
	}

	return buildFiles
}

func detectEntryPoints(dir string, langs []Language) []EntryPoint {
	var eps []EntryPoint

	for _, lang := range langs {
		switch lang.Name {
		case "Go":
			// cmd/ directories
			cmdDir := filepath.Join(dir, "cmd")
			if entries, err := os.ReadDir(cmdDir); err == nil {
				for _, e := range entries {
					if e.IsDir() {
						mainFile := filepath.Join("cmd", e.Name(), "main.go")
						if fileExists(filepath.Join(dir, mainFile)) {
							eps = append(eps, EntryPoint{
								Path:        mainFile,
								Description: e.Name() + " CLI entry point",
							})
						}
					}
				}
			}
			// root main.go
			if fileExists(filepath.Join(dir, "main.go")) {
				eps = append(eps, EntryPoint{
					Path:        "main.go",
					Description: "Main entry point",
				})
			}
		case "TypeScript", "JavaScript":
			for _, f := range []string{"src/index.ts", "src/index.js", "src/main.ts", "src/main.js", "index.ts", "index.js"} {
				if fileExists(filepath.Join(dir, f)) {
					eps = append(eps, EntryPoint{
						Path:        f,
						Description: "Application entry point",
					})
				}
			}
		case "Python":
			for _, f := range []string{"main.py", "app.py", "src/main.py", "manage.py"} {
				if fileExists(filepath.Join(dir, f)) {
					eps = append(eps, EntryPoint{
						Path:        f,
						Description: "Application entry point",
					})
				}
			}
		}
	}
	return eps
}

func detectTestConfig(dir string, langs []Language, buildFiles []BuildFile) TestConfiguration {
	tc := TestConfiguration{}

	for _, lang := range langs {
		switch lang.Name {
		case "Go":
			tc.Framework = "go test"
			tc.Command = "go test -race ./..."
			tc.Dirs = findDirsWithSuffix(dir, "_test.go")
			// Check for coverage in Makefile
			for _, bf := range buildFiles {
				for _, cmd := range bf.Commands {
					if strings.Contains(cmd, "-cover") || strings.Contains(cmd, "--cov") {
						tc.CoverageOn = true
						break
					}
				}
			}
		case "TypeScript", "JavaScript":
			for _, bf := range buildFiles {
				if cmd, ok := bf.Commands["test"]; ok {
					tc.Command = "npm test"
					if strings.Contains(cmd, "jest") {
						tc.Framework = "Jest"
					} else if strings.Contains(cmd, "vitest") {
						tc.Framework = "Vitest"
					} else if strings.Contains(cmd, "mocha") {
						tc.Framework = "Mocha"
					} else {
						tc.Framework = "npm test"
					}
					break
				}
			}
			for _, d := range []string{"test", "tests", "__tests__", "spec"} {
				if fileExists(filepath.Join(dir, d)) {
					tc.Dirs = append(tc.Dirs, d)
				}
			}
		case "Python":
			tc.Framework = "pytest"
			tc.Command = "pytest"
			for _, d := range []string{"tests", "test"} {
				if fileExists(filepath.Join(dir, d)) {
					tc.Dirs = append(tc.Dirs, d)
				}
			}
		}
		if tc.Framework != "" {
			break
		}
	}
	return tc
}

func scanDirectoryTree(dir string, depth int) []DirEntry {
	if depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var tree []DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip hidden dirs and common non-essential dirs
		if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
			continue
		}

		rel, _ := filepath.Rel(filepath.Dir(dir), filepath.Join(dir, name))
		if depth == 0 {
			rel = name
		}

		entry := DirEntry{
			Name:        name,
			Path:        rel,
			Description: inferDirDescription(name),
			Children:    scanDirectoryTree(filepath.Join(dir, name), depth+1),
		}
		tree = append(tree, entry)
	}
	return tree
}

// parseMakefileTargets extracts all target names and their first recipe line from a Makefile.
// It parses both simple and .PHONY targets, skips internal/dot-prefixed targets,
// and handles variable assignments and multi-line recipes.
func parseMakefileTargets(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	targets := make(map[string]string)
	phonyTargets := make(map[string]bool)

	scanner := bufio.NewScanner(f)
	var currentTarget string
	var currentRecipe []string

	flushTarget := func() {
		if currentTarget != "" && len(currentRecipe) > 0 {
			// Store actual recipe for the target
			recipe := strings.Join(currentRecipe, " && ")
			// Strip @ prefix (silent execution marker)
			recipe = strings.TrimPrefix(recipe, "@")
			targets[currentTarget] = recipe
		} else if currentTarget != "" {
			// Target with no meaningful recipe — use make <target>
			targets[currentTarget] = "make " + currentTarget
		}
		currentTarget = ""
		currentRecipe = nil
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Recipe line (starts with tab)
		if strings.HasPrefix(line, "\t") {
			recipe := strings.TrimSpace(line)
			if currentTarget != "" && recipe != "" && !strings.HasPrefix(recipe, "#") {
				// Skip echo-only lines that are just logging
				if !strings.HasPrefix(recipe, "@echo") && !strings.HasPrefix(recipe, "echo ") {
					currentRecipe = append(currentRecipe, strings.TrimPrefix(recipe, "@"))
				}
			}
			continue
		}

		// Non-recipe line — flush previous target
		flushTarget()

		// Skip comments, empty lines, variable assignments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "=") && !strings.Contains(trimmed, ":") {
			continue
		}

		// .PHONY declaration
		if strings.HasPrefix(trimmed, ".PHONY:") {
			phonies := strings.TrimPrefix(trimmed, ".PHONY:")
			for _, p := range strings.Fields(phonies) {
				phonyTargets[p] = true
			}
			continue
		}

		// Skip other dot-directives
		if strings.HasPrefix(trimmed, ".") {
			continue
		}

		// Target line: "target: deps" or "target:"
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			name := strings.TrimSpace(parts[0])
			// Skip targets with spaces (likely variable expansions) or paths
			if name != "" && !strings.Contains(name, " ") && !strings.Contains(name, "/") &&
				!strings.Contains(name, "$") {
				currentTarget = name
			}
		}
	}
	flushTarget()

	return targets
}

func parsePyprojectScripts(path string) map[string]string {
	// Simple TOML parsing for [tool.poetry.scripts] or [project.scripts]
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	cmds := make(map[string]string)
	scanner := bufio.NewScanner(f)
	inScripts := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[tool.poetry.scripts]" || line == "[project.scripts]" {
			inScripts = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inScripts = false
			continue
		}
		if inScripts && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			name := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			cmds[name] = value
		}
	}
	return cmds
}

func findDirsWithSuffix(dir, suffix string) []string {
	seen := make(map[string]bool)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			rel, _ := filepath.Rel(dir, filepath.Dir(path))
			if !seen[rel] {
				seen[rel] = true
			}
		}
		// Limit depth
		rel, _ := filepath.Rel(dir, path)
		if strings.Count(rel, string(filepath.Separator)) > 4 {
			return filepath.SkipDir
		}
		return nil
	})

	var dirs []string
	for d := range seen {
		dirs = append(dirs, d)
	}
	return dirs
}

func inferDirDescription(name string) string {
	descriptions := map[string]string{
		"cmd":        "CLI entry points",
		"pkg":        "Public reusable libraries",
		"internal":   "Private implementation packages",
		"api":        "API definitions and handlers",
		"web":        "Web server and routes",
		"src":        "Source code",
		"lib":        "Library code",
		"test":       "Test files",
		"tests":      "Test files",
		"docs":       "Documentation",
		"scripts":    "Build and utility scripts",
		"config":     "Configuration files",
		"templates":  "Template files",
		"assets":     "Static assets",
		"bin":        "Binary output",
		"build":      "Build output",
		"dist":       "Distribution output",
		"vendor":     "Vendored dependencies",
		"migrations": "Database migrations",
		"proto":      "Protocol buffer definitions",
		"content":    "Content assets",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return ""
}

func isIgnoredDir(name string) bool {
	ignored := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"__pycache__":  true,
		".git":         true,
		"dist":         true,
		"build":        true,
		"target":       true,
		".next":        true,
		".nuxt":        true,
		"coverage":     true,
	}
	return ignored[name]
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mergeMaps(a, b map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range a {
		result[k] = v
	}
	for k, v := range b {
		result[k] = v
	}
	return result
}
