package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func detectFrameworkVersion(dir, framework string) string {
	if version := detectNodeFrameworkVersion(dir, framework); version != "" {
		return version
	}
	switch strings.ToLower(framework) {
	case "gin":
		return detectGoModuleVersion(dir, "github.com/gin-gonic/gin")
	case "echo":
		return detectGoModuleVersion(dir, "github.com/labstack/echo")
	case "chi":
		return detectGoModuleVersion(dir, "github.com/go-chi/chi")
	case "fastapi", "django", "flask":
		return detectPythonDependencyVersion(dir, strings.ToLower(framework))
	case "axum":
		return detectCargoDependencyVersion(dir, "axum")
	default:
		return ""
	}
}

func detectNodeFrameworkVersion(dir, framework string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	allDeps := mergeMaps(pkg.Deps, pkg.DevDeps)
	switch strings.ToLower(framework) {
	case "nextjs":
		return allDeps["next"]
	case "nuxtjs":
		return allDeps["nuxt"]
	case "nestjs":
		return allDeps["@nestjs/core"]
	case "react":
		return allDeps["react"]
	case "vue":
		return allDeps["vue"]
	case "svelte":
		return allDeps["svelte"]
	default:
		return ""
	}
}

func detectGoModuleVersion(dir, module string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		for i, field := range fields {
			if field == module && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return ""
}

func detectPythonDependencyVersion(dir, dep string) string {
	for _, file := range []string{"pyproject.toml", "requirements.txt"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if version := extractDependencyVersion(line, dep); version != "" {
				return version
			}
		}
	}
	return ""
}

func extractDependencyVersion(line, dep string) string {
	clean := strings.Trim(strings.TrimSpace(line), `"',`)
	lower := strings.ToLower(clean)
	if !strings.HasPrefix(lower, dep) {
		return ""
	}
	rest := strings.TrimSpace(clean[len(dep):])
	for _, op := range []string{"==", ">=", "<=", "~=", ">", "<", "="} {
		if strings.HasPrefix(rest, op) {
			return trimDependencyVersion(rest[len(op):])
		}
	}
	return ""
}

func trimDependencyVersion(raw string) string {
	clean := strings.Trim(raw, ` "',]}`)
	for i, r := range clean {
		if r == ',' || r == ' ' || r == '"' || r == '\'' || r == ']' || r == '}' {
			return clean[:i]
		}
	}
	return clean
}

func detectCargoDependencyVersion(dir, dep string) string {
	data, err := os.ReadFile(filepath.Join(dir, "Cargo.toml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		clean := strings.TrimSpace(line)
		if !strings.HasPrefix(clean, dep) || !strings.Contains(clean, "=") {
			continue
		}
		parts := strings.SplitN(clean, "=", 2)
		return trimDependencyVersion(parts[1])
	}
	return ""
}
