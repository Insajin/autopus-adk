package ompprobe

import (
	"os"
	"path/filepath"
	"testing"
)

func callFixture(sequence int) Record {
	return Record{
		Kind:               KindCall,
		Sequence:           sequence,
		Variant:            VariantFull,
		SessionSequence:    sequence,
		SessionSegment:     1,
		StatsBefore:        &Usage{Input: 100, CacheRead: 20, Total: 120},
		StatsAfter:         &Usage{Input: 400, CacheRead: 40, Total: 440},
		TurnUsages:         []Usage{{Input: 300, Output: 12, Total: 312}},
		BranchUsageDelta:   &Usage{Input: -17, Total: -17},
		UsageIdentityDelta: -3,
		ElapsedMS:          1200,
	}
}

func compactionFixture(sequence int) Record {
	before, after := int64(4200), int64(1100)
	maintenanceInput, maintenanceOutput := int64(0), int64(0)
	return Record{
		Kind:                    KindCompaction,
		Sequence:                sequence,
		Variant:                 VariantOptimized,
		SessionSequence:         sequence,
		SessionSegment:          2,
		UsageIdentityDelta:      0,
		ElapsedMS:               980,
		Outcome:                 OutcomeCompleted,
		Method:                  MethodSnapcompact,
		Refusal:                 RefusalNone,
		AttemptCoverage:         AttemptCoverageLocalOnly,
		TokensBefore:            &before,
		TokensAfter:             &after,
		MaintenanceUsagePresent: true,
		MaintenanceUsageKeys:    []string{"inputTokens", "outputTokens", "totalTokens"},
		MaintenanceInputTokens:  &maintenanceInput,
		MaintenanceOutputTokens: &maintenanceOutput,
		CompactionImages:        1,
		UIRequestMethods:        []string{"notify"},
		Roles:                   map[string]int{"assistant": 4, "compactionSummary": 1, "toolResult": 2},
		ContentTypes:            map[string]int{"text": 6, "toolCall": 3, "openaiResponsesHistory": 1},
		RejectedIdentifiers:     2,
	}
}

func fixtureBody(t *testing.T) []byte {
	t.Helper()
	body, err := MarshalRecordSet([]Record{callFixture(1), compactionFixture(2)})
	if err != nil {
		t.Fatalf("marshal fixture records: %v", err)
	}
	return body
}

// fixtureLine returns one serialized record without its newline so a test can
// splice a mutation into an otherwise valid line.
func fixtureLine(t *testing.T, record Record) []byte {
	t.Helper()
	body, err := MarshalRecordSet([]Record{record})
	if err != nil {
		t.Fatalf("marshal fixture record: %v", err)
	}
	return body[:len(body)-1]
}

// realTempDir resolves the temporary directory to its physical path. On macOS
// the per-test directory sits under /var, which is itself a symlink, and the
// exporter refuses to follow any component.
func realTempDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary directory: %v", err)
	}
	return resolved
}

func writeFixtureFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
}

func makeFixtureDirectory(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
}
