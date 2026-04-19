package setup

import (
	"os"
	"path/filepath"
	"strings"
)

func scanDirectoryTree(dir string, depth int) []DirEntry {
	if depth >= maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var tree []DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || isIgnoredDir(name) {
			continue
		}

		rel, _ := filepath.Rel(filepath.Dir(dir), filepath.Join(dir, name))
		if depth == 0 {
			rel = name
		}

		tree = append(tree, DirEntry{
			Name:        name,
			Path:        rel,
			Description: inferDirDescription(name),
			Children:    scanDirectoryTree(filepath.Join(dir, name), depth+1),
		})
	}

	return tree
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
	return descriptions[name]
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
