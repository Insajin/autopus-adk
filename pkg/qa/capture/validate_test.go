package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_AcceptsWellFormedIndex(t *testing.T) {
	t.Parallel()

	index := validIndex(t, t.TempDir())
	require.NoError(t, Validate(index))
	assert.Equal(t, 2, index.Totals.Steps)
	assert.Equal(t, 1, index.Totals.ConsoleErrors)
	assert.Equal(t, 1, index.Totals.NetworkFailures)
	assert.Equal(t, 2, index.Totals.Screenshots)
}

func TestValidate_RejectsContractViolations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*Index)
		want   string
	}{
		{"schema version", func(i *Index) { i.SchemaVersion = "qamesh.gui_capture_index.v9" }, "unsupported schema_version"},
		{"missing journey", func(i *Index) { i.JourneyID = " " }, "missing required field journey_id"},
		{"mode", func(i *Index) { i.Mode = "sometimes" }, "unsupported mode"},
		{"stream", func(i *Index) { i.Streams = append(i.Streams, "keystrokes") }, "unsupported stream"},
		{"step id shape", func(i *Index) { i.Steps[0].StepID = "bad id!" }, "step_id must match"},
		{"duplicate step id", func(i *Index) { i.Steps[1].StepID = i.Steps[0].StepID }, "repeats step_id"},
		{"sparse order", func(i *Index) { i.Steps[1].Order = 5 }, "order must be 2"},
		{"status", func(i *Index) { i.Steps[0].Status = "flaky" }, "is unsupported"},
		{"failure summary required", func(i *Index) { i.Steps[1].FailureSummary = "" }, "failure_summary is required"},
		{"console severity", func(i *Index) { i.Steps[0].Console.Messages[0].Severity = "trace" }, "severity"},
		{"console text", func(i *Index) { i.Steps[0].Console.Messages[0].Text = "  " }, "text is required"},
		{"http method", func(i *Index) { i.Steps[0].Network.Entries[0].Method = "get" }, "uppercase HTTP method"},
		{"status range", func(i *Index) { i.Steps[0].Network.Entries[0].Status = 42 }, "out of range"},
		{"digest", func(i *Index) { i.Steps[0].Screenshot.Digest = "sha1:abc" }, "digest must match"},
		{"retention", func(i *Index) { i.Steps[0].Screenshot.Retention = "publishable" }, `retention must be "local_only"`},
		{"zero bytes", func(i *Index) { i.Steps[0].Screenshot.Bytes = 0 }, "bytes must be positive"},
		{"media kind", func(i *Index) { i.Media[0].Kind = "keylog" }, "kind"},
		{"replay kind", func(i *Index) { i.Replay.Kind = "shell" }, "replay.kind"},
		{"replay digest", func(i *Index) { i.Replay.Digest = "nope" }, "replay.digest must match"},
		{"totals drift", func(i *Index) { i.Totals.ConsoleErrors = 99 }, "totals disagree with steps"},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			index := validIndex(t, t.TempDir())
			testCase.mutate(&index)
			err := Validate(index)
			require.Error(t, err)
			assert.Contains(t, err.Error(), testCase.want)
		})
	}
}

// TestValidate_RejectsRawURLs is the rule that keeps tokens out of network
// evidence: the reference shape itself cannot express a URL.
func TestValidate_RejectsRawURLs(t *testing.T) {
	t.Parallel()

	refs := []string{
		"https://shop.test/api/order",
		"//shop.test/api",
		"/api/order?token=sk-live-abcdefghijklmnop",
		"/api/order#frag",
		"user:pass@shop.test/api",
		"api/order",
		"origin:999999/api",
	}
	for _, ref := range refs {
		ref := ref
		t.Run(ref, func(t *testing.T) {
			t.Parallel()
			index := validIndex(t, t.TempDir())
			index.Steps[0].Network.Entries[0].URLRef = ref
			require.Error(t, Validate(index))
		})
	}
}

func TestValidate_RejectsEscapingMediaRefs(t *testing.T) {
	t.Parallel()

	for _, ref := range []string{"../../etc/passwd", "/etc/passwd", "..", "screenshots/../../out.png"} {
		ref := ref
		t.Run(ref, func(t *testing.T) {
			t.Parallel()
			index := validIndex(t, t.TempDir())
			index.Steps[0].Screenshot.Ref = ref
			err := Validate(index)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "capture directory")
		})
	}
}

func TestValidate_RejectsShellMetacharactersInReplay(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{"test; rm -rf /", "test && curl x", "$(whoami)", "a|b", "back`tick`"} {
		arg := arg
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			index := validIndex(t, t.TempDir())
			index.Replay.Command = append(index.Replay.Command, arg)
			err := Validate(index)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "shell metacharacters")
		})
	}
}

func TestLoadIndex_IsStrict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	index := validIndex(t, dir)
	body, err := json.Marshal(index)
	require.NoError(t, err)
	path := filepath.Join(dir, IndexFileName)
	require.NoError(t, os.WriteFile(path, body, 0o644))

	loaded, err := LoadIndex(path)
	require.NoError(t, err)
	assert.Equal(t, index.JourneyID, loaded.JourneyID)
	require.Len(t, loaded.Steps, 2)

	// An unknown field means producer and harness disagree about what was
	// captured; silently ignoring it is how evidence used to disappear.
	_, err = DecodeIndex([]byte(`{"schema_version":"qamesh.gui_capture_index.v1","surprise":1}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "surprise")

	// A duplicate key could hide a failing step behind a passing one.
	_, err = DecodeIndex([]byte(`{"journey_id":"a","journey_id":"b"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")

	_, err = DecodeIndex([]byte(`{"journey_id":"a"} {"journey_id":"b"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing content")
}

func TestLoadIndex_RejectsOversizedDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, IndexFileName)
	require.NoError(t, os.WriteFile(path, make([]byte, MaxIndexBytes+1), 0o644))
	_, err := LoadIndex(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}
