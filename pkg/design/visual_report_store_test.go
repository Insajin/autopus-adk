package design

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVisualGateReportBundle_RepeatedWrites_ReadsLatestCommittedGeneration(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteVisualGateReportBundle(
		root,
		VisualGateReport{Version: 1, GeneratedAt: "generation-1", Verdict: "WARN"},
		VisualGateReportV2{Version: 2, GeneratedAt: "generation-1", Verdict: "WARN"},
	))
	require.NoError(t, WriteVisualGateReportBundle(
		root,
		VisualGateReport{Version: 1, GeneratedAt: "generation-2", Verdict: "FAIL"},
		VisualGateReportV2{Version: 2, GeneratedAt: "generation-2", Verdict: "FAIL"},
	))

	legacy, evidence, err := ReadVisualGateReportBundle(root)
	require.NoError(t, err)
	assert.Equal(t, "generation-2", legacy.GeneratedAt)
	assert.Equal(t, "generation-2", evidence.GeneratedAt)
	assertNoVisualReportTemps(t, root)
}

func TestWriteVisualGateReport_RelativeRoot_PreservesRelativeReturnPath(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	root, err := filepath.Rel(cwd, t.TempDir())
	require.NoError(t, err)

	path, err := WriteVisualGateReport(root, VisualGateReport{Version: 1})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, filepath.FromSlash(visualReportV1Path)), path)
}

func TestWriteVisualGateReport_MissingRoot_CreatesWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "new-workspace")

	path, err := WriteVisualGateReport(root, VisualGateReport{Version: 1})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, filepath.FromSlash(visualReportV1Path)), path)
	assert.FileExists(t, path)
}

func TestWriteVisualGateReport_SymlinkWorkspaceRoot_WritesIntoResolvedTarget(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "workspace")
	require.NoError(t, os.Mkdir(target, 0o755))
	root := filepath.Join(parent, "workspace-link")
	require.NoError(t, os.Symlink(target, root))

	path, err := WriteVisualGateReport(root, VisualGateReport{Version: 1})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, filepath.FromSlash(visualReportV1Path)), path)
	assert.FileExists(t, filepath.Join(target, filepath.FromSlash(visualReportV1Path)))
}

func TestReadVisualGateReportBundle_MissingRoot_DoesNotCreateWorkspace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-workspace")

	_, _, err := ReadVisualGateReportBundle(root)
	require.Error(t, err)
	assert.NoDirExists(t, root)
}

func TestWriteVisualGateReportBundle_SecondRenameFails_RejectsStaleV2AndCleansTemps(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteVisualGateReportBundle(
		root,
		VisualGateReport{Version: 1, GeneratedAt: "generation-1", Verdict: "WARN"},
		VisualGateReportV2{Version: 2, GeneratedAt: "generation-1", Verdict: "WARN"},
	))
	v2Path := filepath.Join(root, visualReportV2Path)
	oldV2, err := os.ReadFile(v2Path)
	require.NoError(t, err)

	rootFS, err := os.OpenRoot(root)
	require.NoError(t, err)
	defer func() { _ = rootFS.Close() }()
	failing := &failSecondRenameRoot{visualReportRoot: rootFS}
	err = writeVisualGateReportBundleRoot(
		failing,
		VisualGateReport{Version: 1, GeneratedAt: "generation-2", Verdict: "FAIL"},
		VisualGateReportV2{Version: 2, GeneratedAt: "generation-2", Verdict: "FAIL"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected second rename failure")

	latestV1 := readJSONMap(t, filepath.Join(root, visualReportV1Path))
	assert.Equal(t, "generation-2", latestV1["generated_at"])
	currentV2, readErr := os.ReadFile(v2Path)
	require.NoError(t, readErr)
	assert.Equal(t, oldV2, currentV2)
	_, untrustedV2, readErr := ReadVisualGateReportBundle(root)
	require.ErrorIs(t, readErr, ErrVisualGateBundleUncommitted)
	assert.Zero(t, untrustedV2)
	assertNoVisualReportTemps(t, root)
}

func TestWriteVisualGateReportBundle_WrongReportVersion_DoesNotPublishBundle(t *testing.T) {
	tests := []struct {
		name     string
		legacy   VisualGateReport
		evidence VisualGateReportV2
	}{
		{
			name:     "wrong v1 version",
			legacy:   VisualGateReport{Version: 9},
			evidence: VisualGateReportV2{Version: 2},
		},
		{
			name:     "wrong v2 version",
			legacy:   VisualGateReport{Version: 1},
			evidence: VisualGateReportV2{Version: 9},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			err := WriteVisualGateReportBundle(root, test.legacy, test.evidence)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "version")
			assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(visualReportV1Path)))
			assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(visualReportV2Path)))
		})
	}
}

func TestReadVisualGateReportBundle_ReportExceedsSizeLimit_RejectsBeforeDecode(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, WriteVisualGateReportBundle(
		root,
		VisualGateReport{Version: 1},
		VisualGateReportV2{Version: 2},
	))
	v2Path := filepath.Join(root, filepath.FromSlash(visualReportV2Path))
	require.NoError(t, os.Truncate(v2Path, maxVisualReportBytes+1))

	_, _, err := ReadVisualGateReportBundle(root)
	require.ErrorIs(t, err, ErrVisualGateBundleUncommitted)
	assert.Contains(t, err.Error(), "size limit")
}

func TestWriteVisualGateReportBundle_V2ExceedsSizeLimit_DoesNotPublishBundle(t *testing.T) {
	root := t.TempDir()
	evidence := VisualGateReportV2{
		Version:       2,
		PlaywrightErr: strings.Repeat("x", int(maxVisualReportBytes)),
	}

	err := WriteVisualGateReportBundle(root, VisualGateReport{Version: 1}, evidence)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size limit")
	assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(visualReportV1Path)))
	assert.NoFileExists(t, filepath.Join(root, filepath.FromSlash(visualReportV2Path)))
	assertNoVisualReportTemps(t, root)
}

func TestValidateOpenedVisualRoot_DifferentDirectory_RejectsRootSwap(t *testing.T) {
	expected := t.TempDir()
	actual := t.TempDir()
	expectedInfo, err := os.Lstat(expected)
	require.NoError(t, err)
	actualRoot, err := os.OpenRoot(actual)
	require.NoError(t, err)
	defer func() { _ = actualRoot.Close() }()

	err = validateOpenedVisualRoot(expectedInfo, actualRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed while opening")
}

type failSecondRenameRoot struct {
	visualReportRoot
	renames int
}

func (root *failSecondRenameRoot) Rename(oldname, newname string) error {
	root.renames++
	if root.renames == 2 {
		return errors.New("injected second rename failure")
	}
	return root.visualReportRoot.Rename(oldname, newname)
}

func assertNoVisualReportTemps(t *testing.T, root string) {
	t.Helper()
	temps, err := filepath.Glob(filepath.Join(root, visualReportDir, ".visual-report-*"))
	require.NoError(t, err)
	assert.Empty(t, temps)
}
