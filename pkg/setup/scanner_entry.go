package setup

import (
	"os"
	"path/filepath"
	"strings"
)

func detectEntryPoints(dir string, langs []Language) []EntryPoint {
	var entryPoints []EntryPoint

	for _, lang := range langs {
		switch lang.Name {
		case "Go":
			cmdDir := filepath.Join(dir, "cmd")
			if entries, err := os.ReadDir(cmdDir); err == nil {
				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}
					mainFile := filepath.Join("cmd", entry.Name(), "main.go")
					if fileExists(filepath.Join(dir, mainFile)) {
						entryPoints = append(entryPoints, EntryPoint{
							Path:        mainFile,
							Description: entry.Name() + " CLI entry point",
						})
					}
				}
			}
			if fileExists(filepath.Join(dir, "main.go")) {
				entryPoints = append(entryPoints, EntryPoint{
					Path:        "main.go",
					Description: "Main entry point",
				})
			}
		case "TypeScript", "JavaScript":
			for _, file := range []string{"src/index.ts", "src/index.js", "src/main.ts", "src/main.js", "index.ts", "index.js"} {
				if fileExists(filepath.Join(dir, file)) {
					entryPoints = append(entryPoints, EntryPoint{
						Path:        file,
						Description: "Application entry point",
					})
				}
			}
		case "Python":
			for _, file := range []string{"main.py", "app.py", "src/main.py", "manage.py"} {
				if fileExists(filepath.Join(dir, file)) {
					entryPoints = append(entryPoints, EntryPoint{
						Path:        file,
						Description: "Application entry point",
					})
				}
			}
		}
	}

	return entryPoints
}

func detectTestConfig(dir string, langs []Language, buildFiles []BuildFile) TestConfiguration {
	config := TestConfiguration{}

	for _, lang := range langs {
		switch lang.Name {
		case "Go":
			config.Framework = "go test"
			config.Command = "go test -race ./..."
			config.Dirs = findDirsWithSuffix(dir, "_test.go")
			for _, buildFile := range buildFiles {
				for _, command := range buildFile.Commands {
					if strings.Contains(command, "-cover") || strings.Contains(command, "--cov") {
						config.CoverageOn = true
						break
					}
				}
			}
		case "TypeScript", "JavaScript":
			for _, buildFile := range buildFiles {
				command, ok := buildFile.Commands["test"]
				if !ok {
					continue
				}
				config.Command = "npm test"
				switch {
				case strings.Contains(command, "jest"):
					config.Framework = "Jest"
				case strings.Contains(command, "vitest"):
					config.Framework = "Vitest"
				case strings.Contains(command, "mocha"):
					config.Framework = "Mocha"
				default:
					config.Framework = "npm test"
				}
				break
			}
			for _, dirName := range []string{"test", "tests", "__tests__", "spec"} {
				if fileExists(filepath.Join(dir, dirName)) {
					config.Dirs = append(config.Dirs, dirName)
				}
			}
		case "Python":
			config.Framework = "pytest"
			config.Command = "pytest"
			for _, dirName := range []string{"tests", "test"} {
				if fileExists(filepath.Join(dir, dirName)) {
					config.Dirs = append(config.Dirs, dirName)
				}
			}
		}
		if config.Framework != "" {
			break
		}
	}

	return config
}

func findDirsWithSuffix(dir, suffix string) []string {
	seen := make(map[string]bool)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			rel, _ := filepath.Rel(dir, filepath.Dir(path))
			seen[rel] = true
		}
		rel, _ := filepath.Rel(dir, path)
		if strings.Count(rel, string(filepath.Separator)) > 4 {
			return filepath.SkipDir
		}
		return nil
	})

	var dirs []string
	for dirName := range seen {
		dirs = append(dirs, dirName)
	}
	return dirs
}
