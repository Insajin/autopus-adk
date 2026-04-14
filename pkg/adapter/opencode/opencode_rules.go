package opencode

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func (a *Adapter) prepareRuleMappings() ([]adapter.FileMapping, error) {
	entries, err := contentfs.FS.ReadDir("rules")
	if err != nil {
		return nil, fmt.Errorf("rules 디렉터리 읽기 실패: %w", err)
	}

	files := make([]adapter.FileMapping, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, readErr := fs.ReadFile(contentfs.FS, filepath.Join("rules", entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("rule 파일 읽기 실패 %s: %w", entry.Name(), readErr)
		}
		content := pkgcontent.ReplacePlatformReferences(string(data), "opencode")
		relPath := filepath.Join(".opencode", "rules", "autopus", entry.Name())
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}

func managedRulePaths() ([]string, error) {
	entries, err := contentfs.FS.ReadDir("rules")
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		paths = append(paths, toSlash(filepath.Join(".opencode", "rules", "autopus", entry.Name())))
	}
	return paths, nil
}
