package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectBlobVisualEvidenceRequiresBuiltinExpectStepIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category string
		title    string
	}{
		{name: "test step spoof", category: "test.step", title: `Expect "toHaveScreenshot(home-default.png)"`},
		{name: "near match title", category: "expect", title: `Expect "toHaveScreenshot(home-default.png)" trailing`},
		{name: "custom expect message", category: "expect", title: "home visual is correct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := successfulScreenshotBlobEvents("/project", ".autopus/baselines/visual")
			step := events[5]["params"].(map[string]any)["step"].(map[string]any)
			step["category"] = tt.category
			step["title"] = tt.title

			evidence := collectVisualEvidence(buildBlobReport(t, events, nil))
			assert.Empty(t, evidence.Assertions)
		})
	}
}

func TestCollectBlobVisualEvidenceAdoptsOnlyFinalRetry(t *testing.T) {
	t.Parallel()

	events := retryBlobEvents([]blobAttemptFixture{
		{resultID: "result-first", retry: 0, status: "failed", attachment: "first-actual.png"},
		{resultID: "result-final", retry: 1, status: "passed"},
	})
	blob := buildBlobReport(t, events, map[string][]byte{"first-actual.png": []byte("old")})
	evidence := collectVisualEvidence(blob)

	assert.Empty(t, evidence.Artifacts, "failed prior retry attachments must be discarded")
	require.Len(t, evidence.Assertions, 1)
	assert.Equal(t, "PASS", evidence.Assertions[0].Status)
	encoded, err := json.Marshal(evidence.Assertions[0])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"result_id":"result-final"`)
	assert.Contains(t, string(encoded), `"retry":1`)
}

func TestCollectBlobVisualEvidenceKeepsOnlyFinalRetryArtifacts(t *testing.T) {
	t.Parallel()

	events := retryBlobEvents([]blobAttemptFixture{
		{resultID: "result-first", retry: 0, status: "failed", attachment: "first-actual.png"},
		{resultID: "result-final", retry: 1, status: "failed", attachment: "final-actual.png"},
	})
	entries := map[string][]byte{"first-actual.png": []byte("old"), "final-actual.png": []byte("new")}
	evidence := collectVisualEvidence(buildBlobReport(t, events, entries))

	require.Len(t, evidence.Artifacts, 1)
	assert.Equal(t, "final-actual.png", evidence.Artifacts[0].Path)
	encoded, err := json.Marshal(evidence.Artifacts[0])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"result_id":"result-final"`)
	assert.Contains(t, string(encoded), `"retry":1`)
}

func TestCollectBlobVisualEvidenceSupportsAnonymousAndNestedScreenshotNames(t *testing.T) {
	t.Parallel()

	events := multiStepBlobEvents([]string{
		`Expect "toHaveScreenshot"`,
		`Expect "toHaveScreenshot"`,
		`Expect "toHaveScreenshot(sections/home/default.png)"`,
		`Expect "toHaveScreenshot(sections\settings\default.png)"`,
		`Expect "toHaveScreenshot(../private.png)"`,
	})
	evidence := collectVisualEvidence(buildBlobReport(t, events, nil))

	require.Len(t, evidence.Assertions, 4)
	assert.Equal(t, "anonymous-screenshot-1.png", evidence.Assertions[0].Name)
	assert.Equal(t, "anonymous-screenshot-2.png", evidence.Assertions[1].Name)
	assert.True(t, evidence.Assertions[0].Anonymous)
	assert.True(t, evidence.Assertions[1].Anonymous)
	assert.Equal(t, "PASS", evidence.Assertions[0].Status)
	assert.Equal(t, "PASS", evidence.Assertions[1].Status)
	assert.Equal(t, "sections/home/default.png", evidence.Assertions[2].Name)
	assert.Equal(t, "sections/settings/default.png", evidence.Assertions[3].Name)
	assert.False(t, evidence.Assertions[2].Anonymous)
	assert.False(t, evidence.Assertions[3].Anonymous)
	assert.Equal(t, "PASS", evidence.Assertions[2].Status)
	assert.Equal(t, "PASS", evidence.Assertions[3].Status)
	assert.NotEqual(t, evidence.Assertions[2].ComparisonID, evidence.Assertions[3].ComparisonID)
}

func TestCollectBlobVisualEvidenceReportsOnlyProjectsThatStartedTests(t *testing.T) {
	t.Parallel()

	events := successfulScreenshotBlobEvents("/project", ".autopus/baselines/visual")
	unused := map[string]any{
		"method": "onProject",
		"params": map[string]any{"project": map[string]any{
			"name": "webkit", "snapshotDir": ".autopus/baselines/visual", "suites": []any{},
		}},
	}
	events = append(events[:3], append([]map[string]any{unused}, events[3:]...)...)

	evidence := collectVisualEvidence(buildBlobReport(t, events, nil))
	assert.Equal(t, []string{"chromium"}, evidence.Projects)
}

func TestBlobCollectorIndexesStepsByResult(t *testing.T) {
	t.Parallel()

	_, ok := reflect.TypeOf(blobCollector{}).FieldByName("stepsByResult")
	assert.True(t, ok, "step lookup must be partitioned by result for linear event processing")
}

func TestCollectBlobVisualEvidence_CorruptSnapshotProof_PreservesReportAsUnproven(t *testing.T) {
	t.Parallel()

	// Given
	events := successfulScreenshotBlobEvents("/project", ".autopus/baselines/visual")
	blob := buildBlobReportBytes(t, encodeBlobEvents(t, events), map[string][]byte{
		snapshotProofEntryName: []byte(`{"version":2,"broken":`),
	})

	// When
	evidence := collectVisualEvidence(blob)

	// Then
	require.Len(t, evidence.Assertions, 1)
	assert.Equal(t, "FAIL", evidence.Assertions[0].Status)
	assert.Equal(t, "unproven", evidence.SnapshotProofStatus)
	assert.Contains(t, evidence.SnapshotProofDiagnostic, "invalid")
}

func encodeBlobEvents(t *testing.T, events []map[string]any) []byte {
	t.Helper()
	var report strings.Builder
	for _, event := range events {
		raw, err := json.Marshal(event)
		require.NoError(t, err)
		report.Write(raw)
		report.WriteByte('\n')
	}
	return []byte(report.String())
}

type blobAttemptFixture struct {
	resultID   string
	retry      int
	status     string
	attachment string
}

func retryBlobEvents(attempts []blobAttemptFixture) []map[string]any {
	base := successfulScreenshotBlobEvents("/project", ".autopus/baselines/visual")
	events := append([]map[string]any{}, base[:4]...)
	for index, attempt := range attempts {
		stepID := "step-" + attempt.resultID
		events = append(events,
			map[string]any{"method": "onTestBegin", "params": map[string]any{
				"testId": "test-home", "result": map[string]any{"id": attempt.resultID, "retry": attempt.retry},
			}},
			map[string]any{"method": "onStepBegin", "params": map[string]any{
				"testId": "test-home", "resultId": attempt.resultID,
				"step": map[string]any{"id": stepID, "title": `Expect "toHaveScreenshot(home-default.png)"`, "category": "expect"},
			}},
			map[string]any{"method": "onStepEnd", "params": map[string]any{
				"testId": "test-home", "resultId": attempt.resultID, "step": map[string]any{"id": stepID},
			}},
		)
		if attempt.attachment != "" {
			events = append(events, map[string]any{"method": "onAttach", "params": map[string]any{
				"testId": "test-home", "resultId": attempt.resultID,
				"attachments": []map[string]any{{"name": "actual", "contentType": "image/png", "path": attempt.attachment}},
			}})
		}
		events = append(events, map[string]any{"method": "onTestEnd", "params": map[string]any{
			"test":   map[string]any{"testId": "test-home"},
			"result": map[string]any{"id": attempt.resultID, "status": attempt.status, "order": index},
		}})
	}
	return append(events, map[string]any{"method": "onEnd", "params": map[string]any{"result": map[string]any{"status": "passed"}}})
}

func multiStepBlobEvents(titles []string) []map[string]any {
	events := successfulScreenshotBlobEvents("/project", ".autopus/baselines/visual")
	prefix := append([]map[string]any{}, events[:5]...)
	for index, title := range titles {
		id := "step-" + string(rune('a'+index))
		prefix = append(prefix,
			map[string]any{"method": "onStepBegin", "params": map[string]any{
				"testId": "test-home", "resultId": "result-home",
				"step": map[string]any{"id": id, "title": title, "category": "expect"},
			}},
			map[string]any{"method": "onStepEnd", "params": map[string]any{
				"testId": "test-home", "resultId": "result-home", "step": map[string]any{"id": id},
			}},
		)
	}
	return append(prefix, events[7:]...)
}
