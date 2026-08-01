package omp

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

const ompDeferredToolsRule = `# OMP Deferred Tool Compatibility

Use only tools exposed by the current oh-my-pi runtime. If a requested tool is
absent, stop that route with an unsupported-tool diagnostic. Persistent Claude
team lifecycle is unavailable here; use the ordinary OMP task pipeline.
`

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
		data, readErr := fs.ReadFile(contentfs.FS, pkgcontent.EmbeddedPath("rules", entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("rule 파일 읽기 실패 %s: %w", entry.Name(), readErr)
		}
		content := pkgcontent.ReplacePlatformReferences(string(data), "omp")
		if entry.Name() == "deferred-tools.md" {
			content = ompDeferredToolsRule
		}

		transformed, transErr := pkgcontent.TransformRuleForOMP(content)
		if transErr != nil {
			return nil, fmt.Errorf("rule 변환 실패 %s: %w", entry.Name(), transErr)
		}
		content = transformed

		relPath := filepath.Join(".agents", "rules", "autopus", entry.Name())
		files = append(files, adapter.FileMapping{
			TargetPath:      relPath,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}
