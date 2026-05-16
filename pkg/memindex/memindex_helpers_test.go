package memindex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

func TestMemIndexTextAndSanitizerHelpers(t *testing.T) {
	t.Parallel()

	raw := "Bearer abcdefghijklmnop /Users/alice/work\x00private_note: hidden"
	redacted := safeText(raw)
	assert.NotContains(t, redacted, "abcdefghijklmnop")
	assert.NotContains(t, redacted, "/Users/alice")
	assert.NotContains(t, redacted, "hidden")
	assert.NotContains(t, redacted, "\x00")
	assert.Contains(t, redacted, "[REDACTED_SECRET]")
	assert.Contains(t, redacted, "[REDACTED_USER]")
	assert.Contains(t, redacted, "[REDACTED_PRIVATE_NOTE]")

	assert.Equal(t, "one two...", compact("  one\n two   three ", 8))
	assert.Equal(t, "Spec Title", titleFromMarkdown("fallback.md", []byte("\n## Spec Title\nbody")))
	assert.Equal(t, "fallback.md", titleFromMarkdown("fallback.md", []byte("body")))
	assert.Equal(t, "first paragraph second paragraph", summaryFromMarkdown([]byte("# Title\n\n| table |\nfirst paragraph\nsecond paragraph\n")))
	assert.Equal(t, "SPEC-PATH-001", detectSpecID(".autopus/specs/SPEC-PATH-001/spec.md", "SPEC-BODY-001"))
	assert.Equal(t, "SPEC-BODY-001", detectSpecID("plain.md", "See SPEC-BODY-001 now"))
	assert.ElementsMatch(t, []string{"spec-auto-mem-001", "spec", "specs"}, tagsFromPath(".autopus/specs/SPEC-AUTO-MEM-001/spec.md"))
	assert.Equal(t, []string{"a", "b"}, uniqueStrings([]string{"a", " ", "a", "b"}))
	assert.Equal(t, "review_failure", sourceKindFromPath(".autopus/project/reviews/review.md"))
	assert.Equal(t, "project_doc", sourceKindFromPath(".autopus/project/overview.md"))
	assert.Equal(t, "spec", sourceKindFromPath(".autopus/specs/SPEC-ONE/spec.md"))
	assert.Equal(t, "document", sourceKindFromPath("README.md"))
	assert.Equal(t, `"approval" OR "drift"`, ftsQuery(`approval "drift" approval`))
	assert.Empty(t, ftsQuery("!!!"))

	assert.Equal(t, []string{"a", "b"}, parseJSONArray(`["a","b"]`))
	assert.Nil(t, parseJSONArray(`{`))
	assert.Equal(t, map[string]any{"a": "b"}, parseJSONMap(`{"a":"b"}`))
	assert.Nil(t, parseJSONMap(`{}`))
	assert.Nil(t, parseJSONMap(`{`))
	assert.LessOrEqual(t, len(snippetDigest(strings.Repeat("x", 300), hashBytes([]byte("source")))), 160)
	assert.Equal(t, `["a","b"]`, jsonArray([]string{"a", "a", "b"}))
	assert.Equal(t, `{}`, jsonMap(nil))
	assert.Equal(t, "a b", joinJSON([]string{"a", "a", "b"}))

	records := sanitizeRecords([]Record{{
		SourceRef:       "ref",
		Title:           "token sk-proj-testsecret000000000000",
		Summary:         "/Users/alice/summary",
		Tags:            []string{"tag", "tag", ""},
		AcceptanceIDs:   []string{"AC-ONE-001", "AC-ONE-001"},
		FileRefs:        []string{"/Users/alice/file.go"},
		PackageRefs:     []string{"pkg/memindex"},
		Severity:        "high",
		RedactionStatus: Redacted,
		Content:         "private_note: hidden",
		SourceMetadata: map[string]any{
			"secret": "sk-proj-metasecret000000000000",
			"nested": map[string]any{"path": "/Users/alice/nested"},
			"list":   []any{"ok", "/Users/alice/list"},
			"strings": []string{
				"dup",
				"dup",
			},
		},
	}})
	require.Len(t, records, 1)
	assert.Equal(t, []string{"tag"}, records[0].Tags)
	assert.Equal(t, []string{"AC-ONE-001"}, records[0].AcceptanceIDs)
	assert.NotContains(t, records[0].Title, "sk-proj")
	assert.NotContains(t, records[0].Summary, "/Users/alice")
	assert.NotContains(t, records[0].Content, "hidden")
	assert.NotContains(t, jsonMap(records[0].SourceMetadata), "/Users/alice")
	assert.NotContains(t, jsonMap(records[0].SourceMetadata), "sk-proj")

	skips := sanitizeSkips([]Skip{{Path: "/Users/alice/raw.md", Reason: "unsafe_source_text"}})
	require.Len(t, skips, 1)
	assert.NotContains(t, skips[0].Path, "/Users/alice")
	assert.Equal(t, map[string]int{"unsafe_source_text": 2}, skippedCounts([]Skip{
		{Reason: "unsafe_source_text"},
		{Reason: "unsafe_source_text"},
	}))

	assert.Contains(t, DefaultIndexPath(""), filepath.Join(".autopus", "runtime", "memindex", "autopus-mem.sqlite"))
	memErr := &Error{Code: "code", Err: errors.New("boom")}
	assert.Equal(t, "code: boom", memErr.Error())
	assert.Equal(t, "boom", (&Error{Err: errors.New("boom")}).Error())
	assert.Equal(t, "", (*Error)(nil).Error())
	assert.True(t, errors.Is(&Error{Err: os.ErrNotExist}, os.ErrNotExist))
}

func TestMemIndexHashPathAndProjectionGuards(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	docPath := filepath.Join(projectDir, ".autopus", "project", "doc.md")
	writeMemIndexFile(t, docPath, "# Doc\n\nfresh hash\n")
	hash, err := hashFile(docPath)
	require.NoError(t, err)
	assert.Equal(t, hashBytes([]byte("# Doc\n\nfresh hash\n")), hash)
	assert.Equal(t, Fresh, sourceFreshness(projectDir, ".autopus/project/doc.md", hash))
	assert.Equal(t, Stale, sourceFreshness(projectDir, ".autopus/project/doc.md", hashBytes([]byte("old"))))
	assert.Equal(t, Missing, sourceFreshness(projectDir, ".autopus/project/missing.md", hash))

	learningPath := filepath.Join(projectDir, ".autopus", "learnings", "pipeline.jsonl")
	writeMemIndexFile(t, learningPath, `{"id":"L-042","pattern":"hash me"}`+"\nnot-json\n")
	learningHash, err := hashLearningLine(learningPath, "L-042")
	require.NoError(t, err)
	current, err := currentSourceHash(projectDir, "L-042")
	require.NoError(t, err)
	assert.Equal(t, learningHash, current)
	current, err = currentSourceHash(projectDir, ".autopus/learnings/pipeline.jsonl#L-042")
	require.NoError(t, err)
	assert.Equal(t, learningHash, current)
	_, err = hashLearningLine(learningPath, "L-404")
	assert.ErrorIs(t, err, os.ErrNotExist)

	assert.Equal(t, ".autopus/project/doc.md", slashRel(projectDir, docPath))
	runtimeRoot := filepath.Join(projectDir, ".autopus", "runtime", "memindex")
	require.NoError(t, os.MkdirAll(runtimeRoot, 0o755))
	ok, err := pathWithinForCreate(runtimeRoot, filepath.Join(runtimeRoot, "nested", "index.sqlite"))
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = pathWithinForCreate(runtimeRoot, filepath.Join(projectDir, "outside.sqlite"))
	require.NoError(t, err)
	assert.False(t, ok)
	parent, err := existingParent(filepath.Join(runtimeRoot, "missing", "child"))
	require.NoError(t, err)
	expectedParent, err := filepath.EvalSymlinks(runtimeRoot)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(expectedParent, "missing", "child"), parent)

	absProject, indexPath, err := normalizePaths(Options{ProjectDir: projectDir, IndexPath: "relative.sqlite"})
	require.NoError(t, err)
	expectedProject, err := filepath.EvalSymlinks(projectDir)
	require.NoError(t, err)
	assert.Equal(t, expectedProject, absProject)
	assert.Equal(t, filepath.Join(expectedParent, "relative.sqlite"), indexPath)
	_, _, err = normalizePaths(Options{ProjectDir: projectDir, IndexPath: filepath.Join(t.TempDir(), "outside.sqlite")})
	require.Error(t, err)
	var memErr *Error
	require.ErrorAs(t, err, &memErr)
	assert.Equal(t, "index-path-outside-runtime", memErr.Code)

	missing, err := Status(Options{ProjectDir: projectDir, IndexPath: "missing.sqlite"})
	require.NoError(t, err)
	assert.True(t, missing.CorruptState.IsCorrupt)
	assert.True(t, missing.RebuildRecommended)

	writeMemIndexFile(t, filepath.Join(runtimeRoot, "corrupt.sqlite"), "not sqlite")
	_, err = Search(SearchOptions{ProjectDir: projectDir, IndexPath: "corrupt.sqlite", Query: "anything"})
	require.Error(t, err)
	require.ErrorAs(t, err, &memErr)
	assert.Equal(t, "projection-corrupt", memErr.Code)
}

func TestMemIndexQAMESHEdgeGuards(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	runDir := filepath.Join(projectDir, ".autopus", "qa", "runs", "qa-edge")
	runIndexPath := filepath.Join(runDir, "run-index.json")

	writeMemIndexFile(t, runIndexPath, "{")
	records, skips, err := qameshIndexRecords(projectDir, runIndexPath)
	require.NoError(t, err)
	assert.Empty(t, records)
	require.Len(t, skips, 1)
	assert.Equal(t, "invalid_qamesh_run_index", skips[0].Reason)

	writeMemIndexJSON(t, runIndexPath, qarun.Index{
		SchemaVersion:   qarun.RunIndexSchemaVersion,
		RunID:           "qa-edge",
		Status:          "failed",
		RedactionStatus: qarun.RedactionStatus{Status: "failed"},
	})
	records, skips, err = qameshIndexRecords(projectDir, runIndexPath)
	require.NoError(t, err)
	assert.Empty(t, records)
	require.Len(t, skips, 1)
	assert.Equal(t, "unredacted_qamesh_run", skips[0].Reason)

	unsafeIndex := qarun.Index{
		RunID:           "qa-unsafe",
		Status:          "failed",
		RedactionStatus: qarun.RedactionStatus{Status: "passed"},
		Checks: []qarun.IndexCheck{{
			ID:             "unsafe",
			Status:         "failed",
			FailureSummary: "sk-proj-edgeunsafe000000000000000",
		}},
	}
	record, ok, skip, err := qameshRunRecord(projectDir, runIndexPath, unsafeIndex)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, record.SourceType)
	assert.Equal(t, "unsafe_source_text", skip.Reason)

	missingManifest := filepath.Join(runDir, "missing.json")
	record, ok, skip, err = qameshManifestRecord(projectDir, missingManifest)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, record.SourceType)
	assert.Equal(t, "missing_qamesh_manifest", skip.Reason)

	writeMemIndexFile(t, filepath.Join(runDir, "invalid-manifest.json"), "{")
	_, ok, skip, err = qameshManifestRecord(projectDir, filepath.Join(runDir, "invalid-manifest.json"))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "invalid_qamesh_manifest", skip.Reason)

	writeMemIndexFile(t, filepath.Join(runDir, "unredacted-manifest.json"), `{"redaction_status":{"status":"failed"}}`)
	_, ok, skip, err = qameshManifestRecord(projectDir, filepath.Join(runDir, "unredacted-manifest.json"))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, "unredacted_qamesh_manifest", skip.Reason)

	resolved, reason := resolveQAMESHRef(projectDir, runDir, "manifest.json")
	assert.Equal(t, filepath.Join(runDir, "manifest.json"), resolved)
	assert.Empty(t, reason)
	_, reason = resolveQAMESHRef(projectDir, runDir, filepath.Join(t.TempDir(), "manifest.json"))
	assert.Equal(t, "outside_configured_roots", reason)

	assert.Equal(t, "high", qameshSeverity("failed"))
	assert.Equal(t, "medium", qameshSeverity("blocked"))
	assert.Equal(t, "low", qameshSeverity("passed"))
	assert.Nil(t, qameshRunDetailRecords(projectDir, filepath.Join(runDir, "missing-index.json"), qarun.Index{}))
	assert.Nil(t, qameshManifestDetailRecords(projectDir, missingManifest, "hash", "2026-05-06T00:00:00Z", "missing.json"))
}
