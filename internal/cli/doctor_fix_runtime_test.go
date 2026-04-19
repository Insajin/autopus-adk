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
	assert.GreaterOrEqual(t, len(calls), 2, "both install and post-install must run")
	assert.Contains(t, out.String(), "post-install complete")
}

func TestRunDoctorFix_PostInstallFailure(t *testing.T) {
	t.Parallel()

	callCount := 0
	mockExec := func(name string, args ...string) ([]byte, error) {
		callCount++
		if callCount == 1 {
			return []byte("ok"), nil
		}
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

	deps := []detect.DependencyStatus{
		makeStatus("playwright", "playwright", "npm i playwright", false, false, "node"),
	}
	ordered := orderByDependency(deps)
	assert.Len(t, ordered, 1)
	assert.Equal(t, "playwright", ordered[0].Name)
}
