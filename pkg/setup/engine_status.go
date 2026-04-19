package setup

import (
	"os"
	"path/filepath"
	"time"
)

// Status returns the documentation status.
type Status struct {
	Exists       bool
	GeneratedAt  time.Time
	FileStatuses map[string]FileStatus
	DriftScore   float64
}

// FileStatus represents the status of a single documentation file.
type FileStatus struct {
	Exists  bool
	Fresh   bool
	ModTime time.Time
}

// GetStatus returns the current documentation status.
func GetStatus(projectDir string, outputDir string) (*Status, error) {
	docsDir := resolveDocsDir(projectDir, outputDir)
	status := &Status{FileStatuses: make(map[string]FileStatus)}

	if _, err := os.Stat(docsDir); err != nil {
		return status, nil
	}
	status.Exists = true

	meta, err := LoadMeta(docsDir)
	if err != nil {
		for _, fileName := range DocFiles {
			docPath := filepath.Join(docsDir, fileName)
			info, statErr := os.Stat(docPath)
			fileStatus := FileStatus{
				Exists: statErr == nil,
				Fresh:  false,
			}
			if statErr == nil {
				fileStatus.ModTime = info.ModTime()
			}
			status.FileStatuses[fileName] = fileStatus
		}
		return status, nil
	}

	status.GeneratedAt = meta.GeneratedAt
	projectInfo, err := Scan(projectDir)
	if err != nil {
		return status, nil
	}

	docContents := renderDocContents(Render(projectInfo, nil))
	staleCount := 0
	for fileName, content := range docContents {
		docPath := filepath.Join(docsDir, fileName)
		info, statErr := os.Stat(docPath)
		fresh := !meta.HasContentChanged(fileName, content)
		if !fresh {
			staleCount++
		}

		fileStatus := FileStatus{
			Exists: statErr == nil,
			Fresh:  fresh,
		}
		if statErr == nil {
			fileStatus.ModTime = info.ModTime()
		}
		status.FileStatuses[fileName] = fileStatus
	}

	if len(docContents) > 0 {
		status.DriftScore = float64(staleCount) / float64(len(docContents))
	}
	return status, nil
}
