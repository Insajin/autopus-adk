package setup

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// collectSourceFiles gathers source files with the given extension, up to maxCount.
func collectSourceFiles(dir, ext string, maxCount int) []string {
	var files []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
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

func detectFileNaming(files []string) string {
	counts := map[string]int{
		"snake_case": 0,
		"kebab-case": 0,
		"camelCase":  0,
		"PascalCase": 0,
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		if strings.Contains(name, "_test") || strings.Contains(name, ".test") || strings.Contains(name, ".spec") {
			continue
		}
		if !strings.ContainsAny(name, "_-") && !hasUpperCase(name[1:]) {
			continue
		}

		switch {
		case strings.Contains(name, "_"):
			counts["snake_case"]++
		case strings.Contains(name, "-"):
			counts["kebab-case"]++
		case len(name) > 0 && unicode.IsUpper(rune(name[0])):
			counts["PascalCase"]++
		case hasUpperCase(name):
			counts["camelCase"]++
		}
	}

	maxCount := 0
	result := "snake_case"
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
