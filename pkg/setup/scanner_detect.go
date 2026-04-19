package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func detectLanguages(dir string) []Language {
	var langs []Language

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

	if fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "setup.py")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) {
		buildFiles := []string{}
		for _, file := range []string{"pyproject.toml", "setup.py", "requirements.txt"} {
			if fileExists(filepath.Join(dir, file)) {
				buildFiles = append(buildFiles, file)
			}
		}
		langs = append(langs, Language{
			Name:       "Python",
			BuildFiles: buildFiles,
		})
	}

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

	if fileExists(filepath.Join(dir, "Makefile")) {
		buildFiles = append(buildFiles, BuildFile{
			Path:     "Makefile",
			Type:     "makefile",
			Commands: parseMakefileTargets(filepath.Join(dir, "Makefile")),
		})
	}

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
