package opencode

import (
	"fmt"
	"path/filepath"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func (a *Adapter) prepareAgentMappings() ([]adapter.FileMapping, error) {
	sources, err := pkgcontent.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	if err != nil {
		return nil, fmt.Errorf("agent source 로드 실패: %w", err)
	}
	files := make([]adapter.FileMapping, 0, len(sources))
	for _, src := range sources {
		content := pkgcontent.TransformAgentForOpenCode(src)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".opencode", "agents", src.Meta.Name+".md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}
