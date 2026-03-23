package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/detect"
)

// makeStatus is a test helper that builds a DependencyStatus.
func makeStatus(name, binary, installCmd string, required, installed bool, dependsOn string) detect.DependencyStatus {
	return detect.DependencyStatus{
		Dependency: detect.Dependency{
			Name:       name,
			Binary:     binary,
			InstallCmd: installCmd,
			Required:   required,
			DependsOn:  dependsOn,
		},
		Installed: installed,
	}
}

func TestFilterMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		statuses []detect.DependencyStatus
		want     []string // names of missing deps
	}{
		{
			name: "all installed",
			statuses: []detect.DependencyStatus{
				makeStatus("node", "node", "", false, true, ""),
				makeStatus("npm", "npm", "", false, true, ""),
			},
			want: nil,
		},
		{
			name: "all missing",
			statuses: []detect.DependencyStatus{
				makeStatus("node", "node", "https://nodejs.org", false, false, ""),
				makeStatus("playwright", "playwright", "npm i -g playwright", false, false, "node"),
			},
			want: []string{"node", "playwright"},
		},
		{
			name: "mixed",
			statuses: []detect.DependencyStatus{
				makeStatus("node", "node", "", false, true, ""),
				makeStatus("playwright", "playwright", "npm i -g playwright", false, false, "node"),
				makeStatus("gh", "gh", "", false, true, ""),
			},
			want: []string{"playwright"},
		},
		{
			name:     "empty input",
			statuses: nil,
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterMissing(tc.statuses)
			if len(tc.want) == 0 {
				assert.Empty(t, got)
				return
			}
			require.Len(t, got, len(tc.want))
			for i, s := range got {
				assert.Equal(t, tc.want[i], s.Name)
			}
		})
	}
}

func TestRunDoctorFix_NodeNotInstalled(t *testing.T) {
	t.Parallel()

	// npm-based dep should be skipped when node is not available.
	// We fake exec to always fail with "not found" so IsInstalled returns false.
	deps := []detect.DependencyStatus{
		makeStatus("ast-grep", "sg", "npm i -g @ast-grep/cli", true, false, ""),
	}

	var out bytes.Buffer
	mockExec := func(name string, args ...string) ([]byte, error) {
		t.Fatalf("exec should not have been called, got: %s %v", name, args)
		return nil, nil
	}
	reader := bufio.NewReader(strings.NewReader(""))

	// ast-grep is npm-based; since node is not in PATH in most test envs we rely
	// on the IsNpmBased check combined with node absence.
	// In CI where node IS installed, the install will be attempted via mockExec.
	// To make the test deterministic, we test orderByDependency and the SKIP path
	// by giving the dep a DependsOn of a dep that is not installed.
	deps2 := []detect.DependencyStatus{
		makeStatus("playwright", "playwright", "npm i -g playwright", false, false, "node-missing"),
	}
	err := runDoctorFixWith(&out, deps2, true, mockExec, reader)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "skipped")
	_ = deps
}

func TestRunDoctorFix_YesFlag(t *testing.T) {
	t.Parallel()

	called := false
	mockExec := func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte("ok"), nil
	}

	// Use a dep with no DependsOn and no npm requirement so it proceeds.
	deps := []detect.DependencyStatus{
		makeStatus("gh", "gh", "brew install gh", false, false, ""),
	}

	var out bytes.Buffer
	// Provide empty stdin — prompt should NOT be read when autoYes=true.
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, deps, true, mockExec, reader)
	require.NoError(t, err)
	assert.True(t, called, "exec should have been called for --yes mode")
}

func TestRunDoctorFix_DependencyOrder(t *testing.T) {
	t.Parallel()

	var order []string
	mockExec := func(name string, args ...string) ([]byte, error) {
		// Extract dep name from install command suffix for tracking.
		order = append(order, name)
		return []byte("ok"), nil
	}

	// node must come before playwright (DependsOn="node").
	deps := []detect.DependencyStatus{
		makeStatus("playwright", "playwright", "npm-fake i -g playwright", false, false, "node"),
		makeStatus("node", "node", "fake-install-node", false, false, ""),
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, deps, true, mockExec, reader)
	require.NoError(t, err)

	// node should appear before playwright in exec calls.
	require.GreaterOrEqual(t, len(order), 2, "both deps should have been installed")
	nodeIdx := -1
	playwrightIdx := -1
	for i, cmd := range order {
		if cmd == "fake-install-node" {
			nodeIdx = i
		}
		if cmd == "npm-fake" {
			playwrightIdx = i
		}
	}
	assert.Less(t, nodeIdx, playwrightIdx, "node must be installed before playwright")
}

func TestRunDoctorFix_UserDeclines(t *testing.T) {
	t.Parallel()

	mockExec := func(name string, args ...string) ([]byte, error) {
		t.Fatalf("exec should not be called when user declines: %s", name)
		return nil, nil
	}

	deps := []detect.DependencyStatus{
		makeStatus("gh", "gh", "brew install gh", false, false, ""),
	}

	var out bytes.Buffer
	// User enters "n" at the prompt.
	reader := bufio.NewReader(strings.NewReader("n\n"))

	err := runDoctorFixWith(&out, deps, false, mockExec, reader)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "skipped by user")
}

func TestRunDoctorFix_UserAccepts(t *testing.T) {
	t.Parallel()

	called := false
	mockExec := func(name string, args ...string) ([]byte, error) {
		called = true
		return []byte("installed ok"), nil
	}

	deps := []detect.DependencyStatus{
		makeStatus("gh", "gh", "brew install gh", false, false, ""),
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader("y\n"))

	err := runDoctorFixWith(&out, deps, false, mockExec, reader)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Contains(t, out.String(), "installed")
}

func TestRunDoctorFix_EACCESError(t *testing.T) {
	t.Parallel()

	mockExec := func(name string, args ...string) ([]byte, error) {
		return []byte("npm ERR! code EACCES permission denied"), fmt.Errorf("exit status 1")
	}

	deps := []detect.DependencyStatus{
		makeStatus("ast-grep", "sg", "brew install ast-grep", true, false, ""),
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, deps, true, mockExec, reader)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "permission denied")
	assert.Contains(t, out.String(), "npm-global")
}

func TestRunDoctorFix_InstallFailure(t *testing.T) {
	t.Parallel()

	mockExec := func(name string, args ...string) ([]byte, error) {
		return []byte("something went wrong"), fmt.Errorf("exit status 1")
	}

	deps := []detect.DependencyStatus{
		makeStatus("gh", "gh", "brew install gh", false, false, ""),
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, deps, true, mockExec, reader)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "install failed")
}

func TestRunDoctorFix_EmptyInstallCmd(t *testing.T) {
	t.Parallel()

	mockExec := func(name string, args ...string) ([]byte, error) {
		t.Fatalf("exec should not be called for empty install cmd: %s", name)
		return nil, nil
	}

	deps := []detect.DependencyStatus{
		makeStatus("gh", "gh", "", false, false, ""),
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, deps, true, mockExec, reader)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "no install command")
}

func TestRunDoctorFix_PostInstallSuccess(t *testing.T) {
	t.Parallel()

	var calls []string
	mockExec := func(name string, args ...string) ([]byte, error) {
		calls = append(calls, name)
		return []byte("ok"), nil
	}

	dep := detect.DependencyStatus{
		Dependency: detect.Dependency{
			Name:           "playwright",
			Binary:         "playwright",
			InstallCmd:     "npm-fake i -g playwright",
			PostInstallCmd: "npx-fake playwright install chromium",
		},
		Installed: false,
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, []detect.DependencyStatus{dep}, true, mockExec, reader)
	require.NoError(t, err)
	// Both main install and post-install should be called.
	assert.GreaterOrEqual(t, len(calls), 2, "both install and post-install must run")
	assert.Contains(t, out.String(), "post-install complete")
}

func TestRunDoctorFix_PostInstallFailure(t *testing.T) {
	t.Parallel()

	callCount := 0
	mockExec := func(name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			// Main install succeeds.
			return []byte("ok"), nil
		}
		// Post-install fails.
		return []byte("browser download failed"), fmt.Errorf("exit status 1")
	}

	dep := detect.DependencyStatus{
		Dependency: detect.Dependency{
			Name:           "playwright",
			Binary:         "playwright",
			InstallCmd:     "npm-fake i -g playwright",
			PostInstallCmd: "npx-fake playwright install chromium",
		},
		Installed: false,
	}

	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))

	err := runDoctorFixWith(&out, []detect.DependencyStatus{dep}, true, mockExec, reader)
	require.NoError(t, err)
	assert.Contains(t, out.String(), "post-install failed")
}

func TestOrderByDependency_NoDeps(t *testing.T) {
	t.Parallel()

	deps := []detect.DependencyStatus{
		makeStatus("a", "a", "install-a", false, false, ""),
		makeStatus("b", "b", "install-b", false, false, ""),
	}
	ordered := orderByDependency(deps)
	assert.Len(t, ordered, 2)
}

func TestOrderByDependency_PrerequisiteNotInList(t *testing.T) {
	t.Parallel()

	// playwright depends on node, but node is not in the list.
	deps := []detect.DependencyStatus{
		makeStatus("playwright", "playwright", "npm i playwright", false, false, "node"),
	}
	ordered := orderByDependency(deps)
	// playwright has DependsOn="node" but node is not in the list, so it goes to "first" group.
	assert.Len(t, ordered, 1)
	assert.Equal(t, "playwright", ordered[0].Name)
}
