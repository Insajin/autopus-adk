package setup

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// sampleGoErrorPatterns extracts unique error handling patterns from Go files.
func sampleGoErrorPatterns(dir string, files []string) []string {
	patterns := make(map[string]int)
	errReturnRe := regexp.MustCompile(`return\s+.*fmt\.Errorf\((.+)\)`)
	errWrapRe := regexp.MustCompile(`return\s+.*errors\.(New|Wrap|Wrapf)\(`)

	for _, fileName := range files {
		if len(patterns) >= 5 {
			break
		}
		path := filepath.Join(dir, fileName)
		file, err := os.Open(path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if errReturnRe.MatchString(line) {
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
	for _, fileName := range files {
		if len(fileName) == 0 {
			continue
		}
		data, err := os.ReadFile(fileName)
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(content, "import (") {
			continue
		}

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
	return "unknown"
}
