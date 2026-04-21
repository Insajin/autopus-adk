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

// ChangePlanMode identifies the setup flow that produced a plan.
type ChangePlanMode string

const (
	ChangePlanModeGenerate ChangePlanMode = "generate"
	ChangePlanModeUpdate   ChangePlanMode = "update"
)

// ChangeAction describes how apply would treat a target file.
type ChangeAction string

const (
	ChangeActionCreate   ChangeAction = "create"
	ChangeActionUpdate   ChangeAction = "update"
	ChangeActionPreserve ChangeAction = "preserve"
	ChangeActionSkip     ChangeAction = "skip"
)

// ChangeClass groups changes by ownership/runtime expectations.
type ChangeClass string

const (
	ChangeClassTrackedDocs      ChangeClass = "tracked_docs"
	ChangeClassGeneratedSurface ChangeClass = "generated_surface"
	ChangeClassRuntimeState     ChangeClass = "runtime_state"
	ChangeClassConfig           ChangeClass = "config"
)

// PlannedChange is a single preview entry in a no-write change plan.
type PlannedChange struct {
	Path   string
	Action ChangeAction
	Class  ChangeClass
	Reason string
}

// WorkspaceHintKind identifies repo-aware context for preview/apply.
type WorkspaceHintKind string

const (
	WorkspaceHintKindSingleRepo WorkspaceHintKind = "single_repo"
	WorkspaceHintKindWorkspace  WorkspaceHintKind = "workspace"
	WorkspaceHintKindMultiRepo  WorkspaceHintKind = "multi_repo"
)

// WorkspaceHint exposes repo-aware context for bootstrap previews.
type WorkspaceHint struct {
	Kind          WorkspaceHintKind
	Repo          string
	SourceOfTruth string
	Message       string
}

// ChangePlan is a reusable no-write preview for setup generate/update flows.
type ChangePlan struct {
	Mode                 ChangePlanMode
	ProjectDir           string
	DocsDir              string
	BuiltAt              time.Time
	Reason               string
	FullRegeneration     bool
	FullRegenerationNote string
	Fingerprint          string
	Changes              []PlannedChange
	WorkspaceHints       []WorkspaceHint

	docSet       *DocSet
	targets      []plannedTarget
	generateOpts GenerateOptions
}

// ApplyResult reports the files written by ApplyChangePlan.
type ApplyResult struct {
	ChangedPaths []string
	DocSet       *DocSet
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

type plannedTarget struct {
	change  PlannedChange
	absPath string
	content []byte
}
