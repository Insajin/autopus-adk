package cli

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

var statusSpecScanSkipDirs = map[string]bool{
	".cache":       true,
	".git":         true,
	"dist":         true,
	"node_modules": true,
	"vendor":       true,
}

// scanAllSpecs scans top-level and nested module SPEC directories.
func scanAllSpecs(baseDir string) []specEntry {
	var all []specEntry

	_ = filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path == baseDir {
			return nil
		}

		name := d.Name()
		if statusSpecScanSkipDirs[name] || (strings.HasPrefix(name, ".") && name != ".autopus") {
			return filepath.SkipDir
		}
		if isGeneratedAutopusMetadataChild(path) {
			return filepath.SkipDir
		}
		if name != "specs" || filepath.Base(filepath.Dir(path)) != ".autopus" {
			return nil
		}

		specs, scanErr := scanSpecs(path)
		if scanErr != nil {
			return filepath.SkipDir
		}
		module, relErr := filepath.Rel(baseDir, filepath.Dir(filepath.Dir(path)))
		if relErr != nil || module == "." {
			module = ""
		} else {
			module = filepath.ToSlash(module)
		}
		for i := range specs {
			specs[i].module = module
		}
		all = append(all, specs...)
		return filepath.SkipDir
	})

	sort.Slice(all, func(i, j int) bool {
		if all[i].module != all[j].module {
			return all[i].module < all[j].module
		}
		return all[i].id < all[j].id
	})
	return all
}

func isGeneratedAutopusMetadataChild(path string) bool {
	return filepath.Base(filepath.Dir(path)) == ".autopus" && filepath.Base(path) != "specs"
}
