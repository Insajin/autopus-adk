package setup

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/version"
)

const metaFileName = ".meta.yaml"

// LoadMeta loads .meta.yaml from the docs directory.
func LoadMeta(docsDir string) (*Meta, error) {
	path := filepath.Join(docsDir, metaFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read meta: %w", err)
	}

	var meta Meta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse meta: %w", err)
	}
	return &meta, nil
}

// SaveMeta writes .meta.yaml to the docs directory.
func SaveMeta(docsDir string, meta *Meta) error {
	data, err := marshalMeta(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(docsDir, metaFileName), data, 0644)
}

// NewMeta creates a Meta with current timestamp and version.
func NewMeta(projectDir string) *Meta {
	return NewMetaAt(projectDir, time.Now().UTC())
}

// NewMetaAt creates a Meta with a fixed timestamp for preview/apply reuse.
func NewMetaAt(projectDir string, generatedAt time.Time) *Meta {
	return &Meta{
		GeneratedAt:    generatedAt.UTC(),
		AutopusVersion: version.Version(),
		ProjectHash:    hashProjectStructure(projectDir),
		Files:          make(map[string]FileMeta),
	}
}

// SetFileMeta records content and source hashes for a document.
func (m *Meta) SetFileMeta(docName, content string, sourceFiles []string, projectDir string) {
	fm := FileMeta{
		ContentHash: hashString(content),
	}
	for _, sf := range sourceFiles {
		absPath := filepath.Join(projectDir, sf)
		if data, err := os.ReadFile(absPath); err == nil {
			fm.SourceHashes = append(fm.SourceHashes, fmt.Sprintf("%s:sha256:%s", sf, hashBytes(data)))
		}
	}
	m.Files[docName] = fm
}

// HasSourceChanged checks if any source files for a document have changed.
func (m *Meta) HasSourceChanged(docName, projectDir string) bool {
	fm, ok := m.Files[docName]
	if !ok {
		return true
	}
	for _, sh := range fm.SourceHashes {
		// Format: "filename:sha256:hash"
		parts := splitSourceHash(sh)
		if len(parts) != 3 {
			return true
		}
		fileName, expectedHash := parts[0], parts[2]
		absPath := filepath.Join(projectDir, fileName)
		data, err := os.ReadFile(absPath)
		if err != nil {
			return true
		}
		if hashBytes(data) != expectedHash {
			return true
		}
	}
	return false
}

// HasContentChanged checks if a document's content has changed.
func (m *Meta) HasContentChanged(docName, content string) bool {
	fm, ok := m.Files[docName]
	if !ok {
		return true
	}
	return fm.ContentHash != hashString(content)
}

func hashString(s string) string {
	return hashBytes([]byte(s))
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func hashProjectStructure(dir string) string {
	// Hash based on top-level directory names and key files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names string
	for _, e := range entries {
		names += e.Name() + "\n"
	}
	return hashString(names)
}

func splitSourceHash(sh string) []string {
	// "filename:sha256:hash" -> ["filename", "sha256", "hash"]
	var parts []string
	// Find first ":"
	for i := 0; i < len(sh); i++ {
		if sh[i] == ':' {
			parts = append(parts, sh[:i])
			rest := sh[i+1:]
			// Find second ":"
			for j := 0; j < len(rest); j++ {
				if rest[j] == ':' {
					parts = append(parts, rest[:j], rest[j+1:])
					return parts
				}
			}
			parts = append(parts, rest)
			return parts
		}
	}
	return []string{sh}
}

func marshalMeta(meta *Meta) ([]byte, error) {
	data, err := yaml.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal meta: %w", err)
	}
	return data, nil
}
