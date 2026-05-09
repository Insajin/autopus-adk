package memindex

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/memindex/driver"
)

func Rebuild(opts Options) (RebuildResult, error) {
	projectDir, indexPath, err := normalizePaths(opts)
	if err != nil {
		return RebuildResult{}, err
	}
	if err := driver.ProbeFTS5(); err != nil {
		return RebuildResult{}, &Error{Code: "auto_mem_fts5_unavailable", Err: err}
	}
	records, skips, err := Scan(projectDir)
	if err != nil {
		return RebuildResult{}, err
	}
	records = sanitizeRecords(records)
	skips = sanitizeSkips(skips)
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return RebuildResult{}, err
	}
	tmp := indexPath + ".tmp"
	_ = os.Remove(tmp)
	db, err := driver.Open(tmp)
	if err != nil {
		return RebuildResult{}, err
	}
	if err := writeProjection(db, records, skips); err != nil {
		_ = db.Close()
		_ = os.Remove(tmp)
		return RebuildResult{}, err
	}
	if err := db.Close(); err != nil {
		_ = os.Remove(tmp)
		return RebuildResult{}, err
	}
	if err := os.Rename(tmp, indexPath); err != nil {
		_ = os.Remove(tmp)
		return RebuildResult{}, err
	}
	return rebuildResult(indexPath, records, skips), nil
}

func writeProjection(db *sql.DB, records []Record, skips []Skip) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("create mem index schema: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`INSERT INTO mem_metadata(key, value) VALUES (?, ?)`, "schema_version", SchemaVersion); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO mem_metadata(key, value) VALUES (?, ?)`, "projection_only", "true"); err != nil {
		return err
	}
	for _, record := range records {
		rowID, err := insertRecord(tx, record)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO mem_records_fts(rowid, title, summary, tags, content) VALUES (?, ?, ?, ?, ?)`,
			rowID, record.Title, record.Summary, joinJSON(record.Tags), record.Content); err != nil {
			return err
		}
	}
	for _, skip := range skips {
		if _, err := tx.Exec(`INSERT INTO mem_skips(path, reason) VALUES (?, ?)`, skip.Path, skip.Reason); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func insertRecord(tx *sql.Tx, record Record) (int64, error) {
	result, err := tx.Exec(`INSERT INTO mem_records(
		source_type, source_ref, source_hash, title, summary, tags, spec_id,
		acceptance_ids, file_refs, package_refs, severity, timestamp,
		redaction_status, content, source_metadata
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.SourceType, record.SourceRef, record.SourceHash, record.Title,
		record.Summary, jsonArray(record.Tags), record.SpecID,
		jsonArray(record.AcceptanceIDs), jsonArray(record.FileRefs),
		jsonArray(record.PackageRefs), record.Severity, record.Timestamp,
		record.RedactionStatus, record.Content, jsonMap(record.SourceMetadata))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func normalizePaths(opts Options) (string, string, error) {
	projectDir := opts.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}
	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return "", "", err
	}
	absProject, err = filepath.EvalSymlinks(absProject)
	if err != nil {
		return "", "", err
	}
	runtimeRoot := filepath.Join(absProject, ".autopus", "runtime", "memindex")
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return "", "", err
	}
	indexPath := opts.IndexPath
	if indexPath == "" {
		indexPath = DefaultIndexPath(absProject)
	} else if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(runtimeRoot, indexPath)
	}
	indexPath = filepath.Clean(indexPath)
	ok, err := pathWithinForCreate(runtimeRoot, indexPath)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", &Error{Code: "index-path-outside-runtime", Err: fmt.Errorf("index path must stay under %s", runtimeRoot)}
	}
	return filepath.Clean(absProject), indexPath, nil
}

func rebuildResult(indexPath string, records []Record, skips []Skip) RebuildResult {
	counts := map[string]int{}
	sourceHashes := map[string]string{}
	indexed := make([]IndexedSource, 0, len(records))
	for _, record := range records {
		counts[record.SourceType]++
		sourceHashes[record.SourceRef] = record.SourceHash
		indexed = append(indexed, IndexedSource{
			SourceType: record.SourceType,
			SourceRef:  record.SourceRef,
			SourceHash: record.SourceHash,
		})
	}
	sort.Slice(indexed, func(i, j int) bool {
		if indexed[i].SourceType != indexed[j].SourceType {
			return indexed[i].SourceType < indexed[j].SourceType
		}
		return indexed[i].SourceRef < indexed[j].SourceRef
	})
	return RebuildResult{
		SchemaVersion:         SchemaVersion,
		ProjectionOnly:        true,
		IndexPath:             indexPath,
		FTS5Verified:          true,
		FTS5Probe:             FTS5Probe{Status: "ok", ProbedBeforeWrite: true},
		CountsBySourceKind:    counts,
		SkippedCountsByReason: skippedCounts(skips),
		SourceHashes:          sourceHashes,
		IndexedSources:        indexed,
		Skipped:               skips,
	}
}

func skippedCounts(skips []Skip) map[string]int {
	counts := map[string]int{}
	for _, skip := range skips {
		counts[skip.Reason]++
	}
	return counts
}

func jsonArray(values []string) string {
	body, _ := json.Marshal(uniqueStrings(values))
	return string(body)
}

func jsonMap(values map[string]any) string {
	if values == nil {
		values = map[string]any{}
	}
	body, _ := json.Marshal(values)
	return string(body)
}

func sanitizeRecords(records []Record) []Record {
	out := make([]Record, 0, len(records))
	for _, record := range records {
		record.SourceRef = safeText(record.SourceRef)
		record.Title = safeText(record.Title)
		record.Summary = safeText(record.Summary)
		record.Tags = sanitizeStrings(record.Tags)
		record.SpecID = safeText(record.SpecID)
		record.AcceptanceIDs = sanitizeStrings(record.AcceptanceIDs)
		record.FileRefs = sanitizeStrings(record.FileRefs)
		record.PackageRefs = sanitizeStrings(record.PackageRefs)
		record.Severity = safeText(record.Severity)
		record.Timestamp = safeText(record.Timestamp)
		record.RedactionStatus = safeText(record.RedactionStatus)
		record.Content = safeText(record.Content)
		record.SourceMetadata = sanitizeMetadata(record.SourceMetadata)
		out = append(out, record)
	}
	return out
}

func sanitizeSkips(skips []Skip) []Skip {
	out := make([]Skip, 0, len(skips))
	for _, skip := range skips {
		out = append(out, Skip{Path: safeText(skip.Path), Reason: safeText(skip.Reason)})
	}
	return out
}

func sanitizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, safeText(value))
	}
	return uniqueStrings(out)
}

func sanitizeMetadata(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[safeText(key)] = sanitizeMetadataValue(value)
	}
	return out
}

func sanitizeMetadataValue(value any) any {
	switch typed := value.(type) {
	case string:
		return safeText(typed)
	case []string:
		return sanitizeStrings(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeMetadataValue(item))
		}
		return out
	case map[string]any:
		return sanitizeMetadata(typed)
	default:
		return typed
	}
}

func joinJSON(values []string) string {
	return strings.Join(uniqueStrings(values), " ")
}
