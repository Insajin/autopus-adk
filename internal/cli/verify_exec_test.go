package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeGitDiff_ReturnsSlice deterministically exercises the success path of
// analyzeGitDiff using a hermetic temp git repo. Two commits guarantee HEAD~1 exists
// regardless of the host's checkout depth, so the changed-file parsing loop is always
// covered (avoids coverage drift between full-history local and shallow CI checkouts).
func TestAnalyzeGitDiff_ReturnsSlice(t *testing.T) {
	// Not parallel: uses os.Chdir.
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.test",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.test",
		)
		require.NoError(t, cmd.Run(), "git %v", args)
	}
	runGit("init")
	require.NoError(t, os.WriteFile("a.txt", []byte("v1\n"), 0o644))
	runGit("add", "a.txt")
	runGit("commit", "-m", "c1")
	require.NoError(t, os.WriteFile("a.txt", []byte("v2\n"), 0o644))
	runGit("add", "a.txt")
	runGit("commit", "-m", "c2")

	files, err := analyzeGitDiff()
	require.NoError(t, err)
	assert.Contains(t, files, "a.txt")
}

// TestAnalyzeGitDiff_NonGitDir deterministically exercises the error path by running
// in a non-git temp directory.
func TestAnalyzeGitDiff_NonGitDir(t *testing.T) {
	// Not parallel: uses os.Chdir.
	dir := t.TempDir()
	orig, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(orig) }()
	require.NoError(t, os.Chdir(dir))

	_, err = analyzeGitDiff()
	assert.Error(t, err, "analyzeGitDiff must error outside a git repository")
}

// TestRunPlaywright_FailsWithoutNpx deterministically exercises the error path by
// emptying PATH so npx is never found, regardless of the host environment.
func TestRunPlaywright_FailsWithoutNpx(t *testing.T) {
	// Not parallel: mutates PATH via t.Setenv.
	t.Setenv("PATH", t.TempDir())

	// npx is absent, so CombinedOutput fails before producing output (out is nil).
	// We only assert on the error path, which is what makes coverage deterministic.
	_, err := runPlaywright("desktop")
	assert.Error(t, err, "runPlaywright must error when npx is absent")
}

func TestRunPlaywrightAllPreservesOpaqueProjectFilter(t *testing.T) {
	run := runPlaywrightWithCapturedArgs(t, "all", "")

	assert.Contains(t, run.Args, "--project=all")
}

func TestRunPlaywrightCommaSeparatedViewportsCreateIndependentProjectFilters(t *testing.T) {
	run := runPlaywrightWithCapturedArgs(t, "desktop,mobile,tablet", "")

	assert.Contains(t, run.Args, "--project=desktop")
	assert.Contains(t, run.Args, "--project=mobile")
	assert.Contains(t, run.Args, "--project=tablet")
	assert.NotContains(t, run.Args, "--project=desktop,mobile,tablet")
}

func TestRunPlaywrightRequestsJSONAndBlobReportersWithUniqueTempOutputs(t *testing.T) {
	first := runPlaywrightWithCapturedArgs(t, "desktop", "")
	second := runPlaywrightWithCapturedArgs(t, "desktop", "")

	reporter := ""
	for _, arg := range first.Args {
		if strings.HasPrefix(arg, "--reporter=") {
			reporter = strings.TrimPrefix(arg, "--reporter=")
		}
	}
	assert.Contains(t, strings.Split(reporter, ","), "json")
	assert.Contains(t, strings.Split(reporter, ","), "blob")
	assert.Contains(t, reporter, "snapshot-proof-reporter.cjs")
	assert.NotEmpty(t, first.BlobOutputPath)
	assert.True(t, filepath.IsAbs(first.BlobOutputPath))
	assert.NotEqual(t, first.BlobOutputPath, second.BlobOutputPath)
}

func TestRunPlaywrightReportsMissingSnapshotComparisonProofWithoutExecutionError(t *testing.T) {
	run, err := runPlaywrightWithCapturedArgsMode(t, "desktop", "", "missing")
	require.NoError(t, err)
	assert.Contains(t, string(run.Output), "unproven")
}

func TestRunPlaywrightReportsDisabledSnapshotComparisonWithoutExecutionError(t *testing.T) {
	run, err := runPlaywrightWithCapturedArgsMode(t, "desktop", "", "disabled")
	require.NoError(t, err)
	assert.Contains(t, string(run.Output), "disabled")
}

func TestRunPlaywright_OversizedBlob_ReturnsHardLimitError(t *testing.T) {
	// Given
	dir := t.TempDir()
	npxPath := filepath.Join(dir, "npx")
	script := `#!/bin/sh
printf '{"version":2,"nonce":"%s","playwright_version":"1.59.1","update_snapshots":"none","projects":[{"name":"chromium","ignore_snapshots":false,"state":"enabled","source":"public"}]}' "$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE" > "$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE"
/usr/bin/truncate -s "$VERIFY_BLOB_SIZE" "$PLAYWRIGHT_BLOB_OUTPUT_FILE"
printf '{"suites":[]}'
`
	require.NoError(t, os.WriteFile(npxPath, []byte(script), 0o755))
	t.Setenv("VERIFY_BLOB_SIZE", strconv.FormatInt(int64(maxBlobArchiveBytes)+1, 10))
	t.Setenv("PATH", dir)

	// When
	_, err := runPlaywright("desktop")

	// Then
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "blob")
	assert.Contains(t, err.Error(), "제한")
}

func TestRunPlaywright_BlobUncompressedEntryBudgetExceeded_ReturnsHardArchiveError(t *testing.T) {
	// Given
	blob := buildRawBlobEntry(t, "report.jsonl", uint64(maxBlobEntryBytes)+1)
	blobPath := filepath.Join(t.TempDir(), "uncompressed-budget.zip")
	require.NoError(t, os.WriteFile(blobPath, blob, 0o600))

	// When
	_, err := runPlaywrightWithCapturedArgsMode(t, "desktop", blobPath, "enabled")

	// Then
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "blob")
	assert.Contains(t, strings.ToLower(err.Error()), "uncompressed")
}

func TestRunPlaywright_MalformedBlob_ReturnsHardArchiveError(t *testing.T) {
	// Given
	blobPath := filepath.Join(t.TempDir(), "malformed.zip")
	require.NoError(t, os.WriteFile(blobPath, []byte("not-a-zip"), 0o600))

	// When
	_, err := runPlaywrightWithCapturedArgsMode(t, "desktop", blobPath, "enabled")

	// Then
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "blob")
}

func TestRunPlaywright_BlobEventBudgetExceeded_ReturnsHardArchiveError(t *testing.T) {
	// Given
	report := bytes.Repeat([]byte("{}\n"), maxBlobEvents+1)
	blob := buildBlobReportBytes(t, report, nil)
	blobPath := filepath.Join(t.TempDir(), "event-budget.zip")
	require.NoError(t, os.WriteFile(blobPath, blob, 0o600))

	// When
	run, err := runPlaywrightWithCapturedArgsMode(t, "desktop", blobPath, "enabled")

	// Then
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "blob")
	assert.Contains(t, strings.ToLower(err.Error()), "event")
	evidence := collectVisualEvidence(run.Output)
	assert.Equal(t, "enabled", evidence.SnapshotProofStatus)
	assert.Equal(t, []string{"chromium"}, evidence.RequiredProjects)
}

func TestRunPlaywrightPrefersBlobEvidenceWhenReporterCreatedIt(t *testing.T) {
	blobPath := filepath.Join(t.TempDir(), "fixture.zip")
	blob := buildBlobReportWithoutProof(t, successfulScreenshotBlobEvents("/private/project", "../../.autopus/baselines/visual"), nil)
	require.NoError(t, os.WriteFile(blobPath, blob, 0o600))

	run := runPlaywrightWithCapturedArgs(t, "desktop", blobPath)
	evidence := collectVisualEvidence(run.Output)

	require.Len(t, evidence.Assertions, 1)
	assert.Equal(t, "home-default.png", evidence.Assertions[0].Name)
}

func TestRunPlaywrightFallsBackToJSONWhenBlobReporterOutputIsAbsent(t *testing.T) {
	run := runPlaywrightWithCapturedArgs(t, "desktop", "")
	evidence := collectVisualEvidence(run.Output)

	require.Len(t, evidence.Artifacts, 1)
	assert.Equal(t, "test-results/fallback.png", evidence.Artifacts[0].Path)
}

type capturedPlaywrightRun struct {
	Args           []string
	BlobOutputPath string
	Output         []byte
}

func runPlaywrightWithCapturedArgs(t *testing.T, viewport, blobFixture string) capturedPlaywrightRun {
	t.Helper()
	run, err := runPlaywrightWithCapturedArgsMode(t, viewport, blobFixture, "enabled")
	require.NoError(t, err)
	return run
}

func runPlaywrightWithCapturedArgsMode(t *testing.T, viewport, blobFixture, proofMode string) (capturedPlaywrightRun, error) {
	t.Helper()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	blobPathCapture := filepath.Join(dir, "blob-path.txt")
	npxPath := filepath.Join(dir, "npx")
	jsonPath := filepath.Join(dir, "fallback.json")
	jsonOutput := []byte(`{"suites":[{"specs":[{"tests":[{"results":[{"attachments":[{"name":"screenshot","contentType":"image/png","path":"test-results/fallback.png"}]}]}]}]}]}`)
	require.NoError(t, os.WriteFile(jsonPath, jsonOutput, 0o600))
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$VERIFY_ARGS_FILE\"\nprintf '%s' \"$PLAYWRIGHT_BLOB_OUTPUT_FILE\" > \"$VERIFY_BLOB_PATH_FILE\"\ncase \"$VERIFY_PROOF_MODE\" in enabled) ignored=false; state=enabled ;; disabled) ignored=true; state=disabled ;; *) state= ;; esac\nif [ -n \"$state\" ]; then printf '{\"version\":2,\"nonce\":\"%s\",\"playwright_version\":\"1.59.1\",\"update_snapshots\":\"none\",\"projects\":[{\"name\":\"chromium\",\"ignore_snapshots\":%s,\"state\":\"%s\",\"source\":\"public\"}]}' \"$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_NONCE\" \"$ignored\" \"$state\" > \"$AUTOPUS_PLAYWRIGHT_SNAPSHOT_PROOF_FILE\"; fi\nif [ -n \"$VERIFY_BLOB_FIXTURE\" ] && [ -n \"$PLAYWRIGHT_BLOB_OUTPUT_FILE\" ]; then /bin/cp \"$VERIFY_BLOB_FIXTURE\" \"$PLAYWRIGHT_BLOB_OUTPUT_FILE\"; fi\n/bin/cat \"$VERIFY_JSON_FIXTURE\"\n"
	require.NoError(t, os.WriteFile(npxPath, []byte(script), 0o755))
	t.Setenv("VERIFY_ARGS_FILE", argsPath)
	t.Setenv("VERIFY_BLOB_PATH_FILE", blobPathCapture)
	t.Setenv("VERIFY_BLOB_FIXTURE", blobFixture)
	t.Setenv("VERIFY_JSON_FIXTURE", jsonPath)
	t.Setenv("VERIFY_PROOF_MODE", proofMode)
	t.Setenv("PATH", dir)

	output, runErr := runPlaywright(viewport)
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		return capturedPlaywrightRun{}, err
	}
	capturedBlobPath, err := os.ReadFile(blobPathCapture)
	if err != nil {
		return capturedPlaywrightRun{}, err
	}
	return capturedPlaywrightRun{
		Args:           strings.Split(strings.TrimSpace(string(raw)), "\n"),
		BlobOutputPath: string(capturedBlobPath),
		Output:         output,
	}, runErr
}
