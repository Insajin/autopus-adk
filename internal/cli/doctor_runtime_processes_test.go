package cli

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindStaleLegacyWorkerMCPProcesses(t *testing.T) {
	restore := stubRuntimeProcessHooks(t,
		[]runtimeProcessRow{
			{PID: 11, Command: "auto worker mcp-serve"},
			{PID: 12, Command: "auto worker mcp-server"},
			{PID: 13, Command: "auto worker mcp-serve"},
			{PID: 14, Command: "auto doctor"},
		},
		map[int]string{
			11: "/Users/me/go/bin/auto.old",
			12: "/usr/local/bin/auto",
			13: "/Users/me/go/bin/auto",
			14: "/Users/me/go/bin/auto.old",
		},
		map[string]error{
			"/usr/local/bin/auto":   os.ErrNotExist,
			"/Users/me/go/bin/auto": nil,
		},
		nil,
	)
	defer restore()

	stale := findStaleLegacyWorkerMCPProcesses()

	require.Len(t, stale, 2)
	assert.Equal(t, 11, stale[0].PID)
	assert.Equal(t, "old auto binary left alive after self-update", stale[0].Reason)
	assert.Equal(t, 12, stale[1].PID)
	assert.Equal(t, "auto binary path no longer exists", stale[1].Reason)
}

func TestCollectRuntimeProcessChecksWarnsOnStaleProcess(t *testing.T) {
	restore := stubRuntimeProcessHooks(t,
		[]runtimeProcessRow{{PID: 21, Command: "auto worker mcp-serve"}},
		map[int]string{21: "/Users/me/go/bin/auto.old"},
		nil,
		nil,
	)
	defer restore()

	report := doctorJSONReport{status: jsonStatusOK}
	report.collectRuntimeProcessChecks(doctorOptions{})

	assert.Equal(t, jsonStatusWarn, report.status)
	require.Len(t, report.data.Runtime, 1)
	assert.Equal(t, 21, report.data.Runtime[0].PID)
	require.Len(t, report.checks, 1)
	assert.Equal(t, "warn", report.checks[0].Status)
	assert.Len(t, report.warnings, 1)
}

func TestCollectRuntimeProcessChecksFixTerminatesStaleProcess(t *testing.T) {
	var terminated []int
	restore := stubRuntimeProcessHooks(t,
		[]runtimeProcessRow{{PID: 31, Command: "auto worker mcp-server"}},
		map[int]string{31: "/Users/me/go/bin/auto.old"},
		nil,
		func(pid int) error {
			terminated = append(terminated, pid)
			return nil
		},
	)
	defer restore()

	report := doctorJSONReport{status: jsonStatusOK}
	report.collectRuntimeProcessChecks(doctorOptions{fix: true})

	assert.Equal(t, jsonStatusOK, report.status)
	assert.Equal(t, []int{31}, terminated)
	require.Len(t, report.checks, 1)
	assert.Equal(t, "pass", report.checks[0].Status)
}

func stubRuntimeProcessHooks(
	t *testing.T,
	rows []runtimeProcessRow,
	executables map[int]string,
	statErrors map[string]error,
	terminate func(pid int) error,
) func() {
	t.Helper()

	origList := listRuntimeProcesses
	origExecutable := runtimeProcessExecutable
	origStat := statRuntimeExecutable
	origTerminate := terminateRuntimeProcessID

	listRuntimeProcesses = func() ([]runtimeProcessRow, error) {
		return rows, nil
	}
	runtimeProcessExecutable = func(pid int) (string, error) {
		return executables[pid], nil
	}
	statRuntimeExecutable = func(path string) (os.FileInfo, error) {
		if err, ok := statErrors[path]; ok {
			return nil, err
		}
		return fakeFileInfo{}, nil
	}
	if terminate != nil {
		terminateRuntimeProcessID = terminate
	}

	return func() {
		listRuntimeProcesses = origList
		runtimeProcessExecutable = origExecutable
		statRuntimeExecutable = origStat
		terminateRuntimeProcessID = origTerminate
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "auto" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
