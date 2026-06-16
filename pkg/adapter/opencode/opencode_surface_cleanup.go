package opencode

import (
	"path/filepath"
)

func opencodePruneRoots() []string {
	return []string{
		filepath.ToSlash(filepath.Join(".agents", "skills")),
		filepath.ToSlash(filepath.Join(".opencode", "skills")),
	}
}
