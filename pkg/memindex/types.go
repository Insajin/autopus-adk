package memindex

import "path/filepath"

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-AUTO-MEM-001: SchemaVersion and exported result structs define the memory projection JSON contract.
// @AX:REASON: CLI JSON output, SQLite metadata validation, and compatibility facades rely on these field names and version semantics.
const (
	SchemaVersion = "autopus.mem_index.v1"
	Redacted      = "redacted"
	Fresh         = "fresh"
	Stale         = "stale"
	Missing       = "missing"
)

type Options struct {
	ProjectDir string
	IndexPath  string
}

type Record struct {
	SourceType      string
	SourceRef       string
	SourceHash      string
	Title           string
	Summary         string
	Tags            []string
	SpecID          string
	AcceptanceIDs   []string
	FileRefs        []string
	PackageRefs     []string
	Severity        string
	Timestamp       string
	RedactionStatus string
	Content         string
	SourceMetadata  map[string]any
}

type Skip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type IndexedSource struct {
	SourceType string `json:"source_type"`
	SourceRef  string `json:"source_ref"`
	SourceHash string `json:"source_hash"`
}

type RebuildResult struct {
	SchemaVersion         string            `json:"schema_version"`
	ProjectionOnly        bool              `json:"projection_only"`
	IndexPath             string            `json:"index_path"`
	FTS5Verified          bool              `json:"fts5_verified"`
	FTS5Probe             FTS5Probe         `json:"fts5_probe"`
	CountsBySourceKind    map[string]int    `json:"counts_by_source_kind"`
	SkippedCountsByReason map[string]int    `json:"skipped_counts_by_reason"`
	SourceHashes          map[string]string `json:"source_hashes"`
	IndexedSources        []IndexedSource   `json:"indexed_sources"`
	Skipped               []Skip            `json:"skipped,omitempty"`
}

type FTS5Probe struct {
	Status            string `json:"status"`
	ProbedBeforeWrite bool   `json:"probed_before_write"`
}

type SearchOptions struct {
	ProjectDir   string
	IndexPath    string
	Query        string
	TopK         int
	RequireFresh bool
}

type SearchResponse struct {
	SchemaVersion string         `json:"schema_version"`
	Query         string         `json:"query"`
	TopK          int            `json:"top_k"`
	Results       []SearchResult `json:"results"`
}

type SearchResult struct {
	Rank            int            `json:"rank"`
	SourceType      string         `json:"source_type"`
	SourceRef       string         `json:"source_ref"`
	SourceHash      string         `json:"source_hash"`
	FreshnessState  string         `json:"freshness_state"`
	SnippetDigest   string         `json:"snippet_digest"`
	RedactionStatus string         `json:"redaction_status"`
	Title           string         `json:"title"`
	Summary         string         `json:"summary"`
	Tags            []string       `json:"tags,omitempty"`
	SpecID          string         `json:"spec_id,omitempty"`
	AcceptanceIDs   []string       `json:"acceptance_ids,omitempty"`
	FileRefs        []string       `json:"file_refs,omitempty"`
	PackageRefs     []string       `json:"package_refs,omitempty"`
	Severity        string         `json:"severity,omitempty"`
	Timestamp       string         `json:"timestamp,omitempty"`
	SourceMetadata  map[string]any `json:"source_metadata,omitempty"`
}

type StatusResult struct {
	SchemaVersion         string         `json:"schema_version"`
	ProjectionOnly        bool           `json:"projection_only"`
	IndexPath             string         `json:"index_path"`
	CountsBySourceKind    map[string]int `json:"counts_by_source_kind"`
	SkippedCountsByReason map[string]int `json:"skipped_counts_by_reason"`
	StaleRefs             []string       `json:"stale_refs"`
	CorruptState          CorruptState   `json:"corrupt_state"`
	RebuildRecommended    bool           `json:"rebuild_recommended"`
}

type CorruptState struct {
	IsCorrupt bool   `json:"is_corrupt"`
	Reason    string `json:"reason,omitempty"`
}

type ContextOptions struct {
	ProjectDir         string
	IndexPath          string
	Query              string
	BudgetTokens       int
	TopK               int
	BuildReceipt       bool
	OutcomeLock        string
	Constraints        []string
	OwnedPaths         []string
	ForbiddenPaths     []string
	AcceptanceCriteria []string
	RequiredReferences []string
	DecisionDelta      string
	SnapshotHash       string
	PromptManifestHash string
}

type ContextResult struct {
	Query              string         `json:"query"`
	BudgetTokens       int            `json:"budget_tokens"`
	RecallBudgetTokens int            `json:"recall_budget_tokens,omitempty"`
	EstimatedTokens    int            `json:"estimated_tokens,omitempty"`
	OmittedCount       int            `json:"omitted_count"`
	Results            []SearchResult `json:"results"`
	Prompt             string         `json:"prompt"`
	OutcomeLock        string         `json:"outcome_lock,omitempty"`
	Constraints        []string       `json:"constraints,omitempty"`
	OwnedPaths         []string       `json:"owned_paths,omitempty"`
	ForbiddenPaths     []string       `json:"forbidden_paths,omitempty"`
	AcceptanceCriteria []string       `json:"acceptance_criteria,omitempty"`
	RequiredReferences []string       `json:"required_references,omitempty"`
	DecisionDelta      string         `json:"decision_delta,omitempty"`
	SnapshotHash       string         `json:"snapshot_hash,omitempty"`
	PromptManifestHash string         `json:"prompt_manifest_hash,omitempty"`
}

type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	if e.Code != "" {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func DefaultIndexPath(projectDir string) string {
	if projectDir == "" {
		projectDir = "."
	}
	return filepath.Join(projectDir, ".autopus", "runtime", "memindex", "autopus-mem.sqlite")
}
