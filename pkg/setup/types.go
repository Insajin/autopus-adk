// Package setup provides project documentation generation and management.
package setup

import "time"

// Language represents a detected programming language.
type Language struct {
	Name       string   // Language name (Go, TypeScript, Python, etc.)
	Version    string   // Detected version (from go.mod, package.json, etc.)
	BuildFiles []string // Associated build files
}

// Framework represents a detected framework or toolkit.
type Framework struct {
	Name    string // Framework name (React, Gin, Django, etc.)
	Version string // Detected version
}

// EntryPoint represents a main entry point of the project.
type EntryPoint struct {
	Path        string // File path relative to project root
	Description string // Brief description
}

// BuildFile represents a build configuration file.
type BuildFile struct {
	Path     string            // File path relative to project root
	Type     string            // Type: makefile, package.json, cargo.toml, go.mod, pyproject.toml, docker-compose
	Commands map[string]string // Extracted commands: name -> command string
}

// TestConfiguration holds test framework and configuration details.
type TestConfiguration struct {
	Framework  string   // Test framework name
	Command    string   // Test execution command
	Dirs       []string // Test directories
	CoverageOn bool     // Whether coverage is configured
}

// DirEntry represents a directory in the project tree.
type DirEntry struct {
	Name        string     // Directory name
	Path        string     // Relative path
	Description string     // Role description
	Children    []DirEntry // Subdirectories (max 3 levels)
}

// ConventionSample holds detected code conventions from actual project files.
type ConventionSample struct {
	FileNaming    string   // Detected file naming pattern: snake_case, kebab-case, camelCase, PascalCase
	ErrorPatterns []string // Sampled error handling patterns from real code
	ImportStyle   string   // Grouped, ungrouped, aliased
	HasLinter     bool     // Whether a linter config exists
	LinterName    string   // Detected linter name
	HasFormatter  bool     // Whether a formatter config exists
	FormatterName string   // Detected formatter name
	ExampleFiles  []string // Paths of representative source files
}

// Workspace represents a monorepo workspace/module.
type Workspace struct {
	Name string // Workspace name or path
	Path string // Relative path to workspace root
	Type string // go.work, npm, cargo, pnpm, yarn
}

// ProjectInfo holds all scanned information about a project.
type ProjectInfo struct {
	Name        string
	RootDir     string
	Languages   []Language
	Frameworks  []Framework
	EntryPoints []EntryPoint
	BuildFiles  []BuildFile
	TestConfig  TestConfiguration
	Structure   []DirEntry                  // Top-level directory tree (max 3 levels)
	Conventions map[string]ConventionSample // Per-language convention samples
	Workspaces  []Workspace                 // Detected monorepo workspaces
	MultiRepo   *MultiRepoInfo              // Detected multi-repo workspace metadata
}

// DocSet holds all rendered documentation content.
type DocSet struct {
	Index        string
	Commands     string
	Structure    string
	Conventions  string
	Boundaries   string
	Architecture string
	Testing      string
	Meta         Meta
}

// DocFiles maps document names to file paths.
var DocFiles = map[string]string{
	"index":        "index.md",
	"commands":     "commands.md",
	"structure":    "structure.md",
	"conventions":  "conventions.md",
	"boundaries":   "boundaries.md",
	"architecture": "architecture.md",
	"testing":      "testing.md",
}

// Meta holds generation metadata for .meta.yaml.
type Meta struct {
	GeneratedAt    time.Time           `yaml:"generated_at"`
	AutopusVersion string              `yaml:"autopus_version"`
	ProjectHash    string              `yaml:"project_hash"`
	Files          map[string]FileMeta `yaml:"files"`
}

// FileMeta holds per-file metadata.
type FileMeta struct {
	ContentHash  string   `yaml:"content_hash"`
	SourceHashes []string `yaml:"source_hashes"`
}

// ValidationReport holds the result of document-code validation.
type ValidationReport struct {
	Valid      bool
	Warnings   []ValidationWarning
	DriftScore float64 // 0.0 = no drift, 1.0 = fully drifted
}

// ValidationWarning represents a single validation issue.
type ValidationWarning struct {
	File    string // Document file
	Line    int    // Line number (0 if unknown)
	Message string // Warning message
	Type    string // stale_path, stale_command, line_limit, missing_lang_id
}

// SetupConfig holds setup-specific configuration from autopus.yaml.
type SetupConfig struct {
	AutoGenerate bool   `yaml:"auto_generate"`
	OutputDir    string `yaml:"output_dir"`
}
