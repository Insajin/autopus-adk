package memindex

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/insajin/autopus-adk/pkg/memindex/driver"
)

func Search(opts SearchOptions) (SearchResponse, error) {
	projectDir, indexPath, err := normalizePaths(Options{ProjectDir: opts.ProjectDir, IndexPath: opts.IndexPath})
	if err != nil {
		return SearchResponse{}, err
	}
	topK := opts.TopK
	if topK <= 0 {
		topK = 10
	}
	db, err := openExisting(indexPath)
	if err != nil {
		return SearchResponse{}, err
	}
	defer func() { _ = db.Close() }()
	if err := verifyProjection(db); err != nil {
		return SearchResponse{}, err
	}
	if opts.RequireFresh {
		stale, err := staleRefs(db, projectDir)
		if err != nil {
			return SearchResponse{}, &Error{Code: "projection-corrupt", Err: err}
		}
		if len(stale) > 0 {
			return SearchResponse{}, &Error{
				Code: "stale-source",
				Err:  fmt.Errorf("source %s is stale; rebuild required", stale[0]),
			}
		}
	}
	results, err := queryRecords(db, projectDir, opts.Query, topK)
	if err != nil {
		return SearchResponse{}, err
	}
	if opts.RequireFresh {
		for _, result := range results {
			if result.FreshnessState != Fresh {
				return SearchResponse{}, &Error{
					Code: "stale-source",
					Err:  fmt.Errorf("source %s is %s; rebuild required", result.SourceRef, result.FreshnessState),
				}
			}
		}
	}
	return SearchResponse{
		SchemaVersion: SchemaVersion,
		Query:         opts.Query,
		TopK:          topK,
		Results:       results,
	}, nil
}

func queryRecords(db *sql.DB, projectDir, query string, topK int) ([]SearchResult, error) {
	match := ftsQuery(query)
	if match == "" {
		return []SearchResult{}, nil
	}
	rows, err := db.Query(`SELECT r.id, r.source_type, r.source_ref, r.source_hash,
		r.title, r.summary, r.tags, r.spec_id, r.acceptance_ids, r.file_refs,
		r.package_refs, r.severity, r.timestamp, r.redaction_status, r.source_metadata
		FROM mem_records_fts f
		JOIN mem_records r ON r.id = f.rowid
		WHERE mem_records_fts MATCH ?
		ORDER BY bm25(mem_records_fts), CASE r.source_type
			WHEN 'learning' THEN 0
			WHEN 'review_failure' THEN 1
			WHEN 'spec' THEN 2
			WHEN 'qamesh_failed_check' THEN 3
			WHEN 'qamesh_setup_gap' THEN 4
			WHEN 'qamesh_repair_prompt' THEN 5
			WHEN 'qamesh_evidence' THEN 6
			WHEN 'qamesh_run' THEN 7
			ELSE 8
		END, r.source_type, r.timestamp DESC, r.source_ref, r.id
		LIMIT ?`, match, topK)
	if err != nil {
		return nil, &Error{Code: "projection-corrupt", Err: err}
	}
	defer func() { _ = rows.Close() }()
	results := make([]SearchResult, 0)
	for rows.Next() {
		var rowID int64
		var result SearchResult
		var tags, acceptanceIDs, fileRefs, packageRefs string
		var metadata string
		if err := rows.Scan(&rowID, &result.SourceType, &result.SourceRef, &result.SourceHash,
			&result.Title, &result.Summary, &tags, &result.SpecID, &acceptanceIDs, &fileRefs,
			&packageRefs, &result.Severity, &result.Timestamp, &result.RedactionStatus, &metadata); err != nil {
			return nil, err
		}
		result.Tags = parseJSONArray(tags)
		result.AcceptanceIDs = parseJSONArray(acceptanceIDs)
		result.FileRefs = parseJSONArray(fileRefs)
		result.PackageRefs = parseJSONArray(packageRefs)
		result.FreshnessState = sourceFreshness(projectDir, result.SourceRef, result.SourceHash)
		result.SnippetDigest = snippetDigest(result.Summary, result.SourceHash)
		result.Rank = len(results) + 1
		result.SourceMetadata = parseJSONMap(metadata)
		_ = rowID
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func Status(opts Options) (StatusResult, error) {
	projectDir, indexPath, err := normalizePaths(opts)
	if err != nil {
		return StatusResult{}, err
	}
	result := StatusResult{
		SchemaVersion:         SchemaVersion,
		ProjectionOnly:        true,
		IndexPath:             indexPath,
		CountsBySourceKind:    map[string]int{},
		SkippedCountsByReason: map[string]int{},
		StaleRefs:             []string{},
	}
	db, err := openExisting(indexPath)
	if err != nil {
		result.CorruptState = CorruptState{IsCorrupt: true, Reason: err.Error()}
		result.RebuildRecommended = true
		return result, nil
	}
	defer func() { _ = db.Close() }()
	if err := verifyProjection(db); err != nil {
		result.CorruptState = CorruptState{IsCorrupt: true, Reason: err.Error()}
		result.RebuildRecommended = true
		return result, nil
	}
	counts, err := countBy(db, `SELECT source_type, COUNT(*) FROM mem_records GROUP BY source_type`)
	if err != nil {
		result.CorruptState = CorruptState{IsCorrupt: true, Reason: err.Error()}
		result.RebuildRecommended = true
		return result, nil
	}
	result.CountsBySourceKind = counts
	skips, err := countBy(db, `SELECT reason, COUNT(*) FROM mem_skips GROUP BY reason`)
	if err != nil {
		result.CorruptState = CorruptState{IsCorrupt: true, Reason: err.Error()}
		result.RebuildRecommended = true
		return result, nil
	}
	result.SkippedCountsByReason = skips
	stale, err := staleRefs(db, projectDir)
	if err != nil {
		result.CorruptState = CorruptState{IsCorrupt: true, Reason: err.Error()}
		result.RebuildRecommended = true
		return result, nil
	}
	result.StaleRefs = stale
	result.RebuildRecommended = len(stale) > 0
	return result, nil
}

func Context(opts ContextOptions) (ContextResult, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}
	budget := opts.BudgetTokens
	if budget <= 0 {
		budget = 800
	}
	response, err := Search(SearchOptions{
		ProjectDir: opts.ProjectDir,
		IndexPath:  opts.IndexPath,
		Query:      opts.Query,
		TopK:       topK,
	})
	if err != nil {
		return ContextResult{}, err
	}
	selected, omitted, prompt := renderContext(opts.Query, response.Results, budget)
	return ContextResult{
		Query:        opts.Query,
		BudgetTokens: budget,
		OmittedCount: omitted,
		Results:      selected,
		Prompt:       prompt,
	}, nil
}

func openExisting(indexPath string) (*sql.DB, error) {
	info, err := os.Stat(indexPath)
	if err != nil {
		return nil, &Error{Code: "rebuild-required", Err: fmt.Errorf("projection missing: %w", err)}
	}
	if info.IsDir() || info.Size() == 0 {
		return nil, &Error{Code: "rebuild-required", Err: fmt.Errorf("projection missing or empty")}
	}
	db, err := driver.OpenReadOnly(indexPath)
	if err != nil {
		return nil, &Error{Code: "projection-corrupt", Err: err}
	}
	return db, nil
}

// @AX:WARN [AUTO]: projection reads fail closed unless schema metadata and FTS table validation pass.
// @AX:REASON: Search/status/context accept configurable SQLite paths and must not query incompatible projections.
func verifyProjection(db *sql.DB) error {
	var version string
	if err := db.QueryRow(`SELECT value FROM mem_metadata WHERE key = 'schema_version'`).Scan(&version); err != nil {
		return &Error{Code: "projection-corrupt", Err: err}
	}
	if version != SchemaVersion {
		return &Error{Code: "rebuild-required", Err: fmt.Errorf("unsupported schema version %q", version)}
	}
	if _, err := db.Exec(`SELECT rowid FROM mem_records_fts LIMIT 1`); err != nil {
		return &Error{Code: "projection-corrupt", Err: err}
	}
	return nil
}

func countBy(db *sql.DB, query string) (map[string]int, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	counts := map[string]int{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, err
		}
		counts[key] = count
	}
	return counts, rows.Err()
}

func staleRefs(db *sql.DB, projectDir string) ([]string, error) {
	rows, err := db.Query(`SELECT source_ref, source_hash FROM mem_records ORDER BY source_ref`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	stale := make([]string, 0)
	for rows.Next() {
		var ref, hash string
		if err := rows.Scan(&ref, &hash); err != nil {
			return nil, err
		}
		if sourceFreshness(projectDir, ref, hash) != Fresh {
			stale = append(stale, ref)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(stale)
	return stale, nil
}

func parseJSONArray(value string) []string {
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil
	}
	return out
}

func parseJSONMap(value string) map[string]any {
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func snippetDigest(summary, sourceHash string) string {
	hash := sourceHash
	if len(hash) > len("sha256:")+12 {
		hash = hash[:len("sha256:")+12]
	}
	return compact(hash+" "+compact(safeText(summary), 120), 160)
}
