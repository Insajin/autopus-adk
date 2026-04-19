package setup

import (
	"github.com/insajin/autopus-adk/pkg/arch"
	"github.com/insajin/autopus-adk/pkg/lore"
)

const (
	maxIndexLines = 200
	maxDocLines   = 500
)

// RenderOptions holds optional data for rendering.
type RenderOptions struct {
	ArchMap   *arch.ArchitectureMap
	LoreItems []lore.LoreEntry
}

// Render generates all documentation files from ProjectInfo.
func Render(info *ProjectInfo, opts *RenderOptions) *DocSet {
	if opts == nil {
		opts = &RenderOptions{}
	}

	ds := &DocSet{
		Index:        renderIndex(info),
		Commands:     renderCommands(info),
		Structure:    renderStructure(info),
		Conventions:  renderConventions(info),
		Boundaries:   renderBoundaries(info),
		Architecture: renderArchitecture(info, opts),
		Testing:      renderTesting(info),
	}
	return ds
}
