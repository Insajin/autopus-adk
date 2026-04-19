package setup

import (
	"fmt"
	"os"
	"path/filepath"
)

func writeDocSet(docsDir, projectDir string, docSet *DocSet, meta *Meta, info *ProjectInfo) error {
	docs := map[string]struct {
		content     string
		sourceFiles []string
	}{
		"index.md":        {docSet.Index, docSourceFiles(info, "index")},
		"commands.md":     {docSet.Commands, docSourceFiles(info, "commands")},
		"structure.md":    {docSet.Structure, docSourceFiles(info, "structure")},
		"conventions.md":  {docSet.Conventions, docSourceFiles(info, "conventions")},
		"boundaries.md":   {docSet.Boundaries, docSourceFiles(info, "boundaries")},
		"architecture.md": {docSet.Architecture, docSourceFiles(info, "architecture")},
		"testing.md":      {docSet.Testing, docSourceFiles(info, "testing")},
	}

	for fileName, doc := range docs {
		path := filepath.Join(docsDir, fileName)
		if err := os.WriteFile(path, []byte(doc.content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", fileName, err)
		}
		meta.SetFileMeta(fileName, doc.content, doc.sourceFiles, projectDir)
	}

	return nil
}

func renderDocContents(docSet *DocSet) map[string]string {
	return map[string]string{
		"index.md":        docSet.Index,
		"commands.md":     docSet.Commands,
		"structure.md":    docSet.Structure,
		"conventions.md":  docSet.Conventions,
		"boundaries.md":   docSet.Boundaries,
		"architecture.md": docSet.Architecture,
		"testing.md":      docSet.Testing,
	}
}

func docSourceFiles(info *ProjectInfo, docType string) []string {
	var files []string
	switch docType {
	case "index", "structure", "commands", "conventions", "boundaries", "architecture", "testing":
		for _, buildFile := range info.BuildFiles {
			files = append(files, buildFile.Path)
		}
	}
	return files
}
