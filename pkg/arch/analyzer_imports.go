package arch

import (
	"bufio"
	"os"
	"strings"
)

// parseGoImports extracts import paths from a Go file.
func parseGoImports(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var imports []string
	inImport := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "import (" {
			inImport = true
			continue
		}
		if inImport && line == ")" {
			inImport = false
			continue
		}
		if inImport {
			imp := strings.TrimSpace(strings.Trim(line, `"`))
			if imp != "" && !strings.HasPrefix(imp, "//") {
				imports = append(imports, imp)
			}
		}
		if strings.HasPrefix(line, `import "`) {
			imp := strings.TrimSuffix(strings.TrimPrefix(line, `import "`), `"`)
			imports = append(imports, imp)
		}
	}
	return imports
}

// parseTSImports extracts import paths from a TypeScript file.
func parseTSImports(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var imports []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import ") && strings.Contains(line, "from ") {
			parts := strings.Split(line, "from ")
			if len(parts) >= 2 {
				imp := strings.Trim(strings.TrimSpace(parts[len(parts)-1]), `'";`)
				if imp != "" {
					imports = append(imports, imp)
				}
			}
		}
		if strings.Contains(line, "require(") {
			start := strings.Index(line, "require(")
			if start >= 0 {
				rest := strings.Trim(line[start+len("require("):], `'"`)
				end := strings.IndexAny(rest, `'"`)
				if end > 0 {
					imports = append(imports, rest[:end])
				}
			}
		}
	}
	return imports
}

// parsePythonImports extracts import paths from a Python file.
func parsePythonImports(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = file.Close() }()

	var imports []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "import ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				imports = append(imports, parts[1])
			}
		}
		if strings.HasPrefix(line, "from ") && strings.Contains(line, " import ") {
			parts := strings.SplitN(line, " import ", 2)
			pkg := strings.TrimSpace(strings.TrimPrefix(parts[0], "from "))
			if pkg != "" {
				imports = append(imports, pkg)
			}
		}
	}
	return imports
}
