package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
)

func TestQARuntimeProvider_FlagRegisteredOnEveryExecutionSurface(t *testing.T) {
	t.Parallel()

	commands := map[string]func() *cobra.Command{
		"run":               newQARunCmd,
		"release":           newQAReleaseCmd,
		"release-readiness": newQAReleaseReadinessCmd,
		"full":              newQAFullCmd,
	}
	for name, factory := range commands {
		name, factory := name, factory
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			flag := factory().Flags().Lookup("runtime-provider")
			require.NotNil(t, flag)
			assert.Equal(t, "stringArray", flag.Value.Type())
		})
	}
}

func TestQARuntimeProvider_MissingObservationSelectionUsesStablePublicCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args func(string, string) []string
	}{
		{name: "run", args: func(dir, output string) []string {
			return []string{"run", "--project-dir", dir, "--lane", "desktop-native", "--journey", "desktop-accessibility-observe", "--adapter", "desktop-accessibility-observe", "--output", output, "--dry-run", "--format", "json"}
		}},
		{name: "release", args: func(dir, output string) []string {
			return []string{"release", "--project-dir", dir, "--output", output, "--dry-run", "--format", "json"}
		}},
		{name: "release-readiness", args: func(dir, _ string) []string {
			return []string{"release-readiness", "--project-dir", dir, "--format", "json"}
		}},
		{name: "full", args: func(dir, output string) []string {
			return []string{"full", "--project-dir", dir, "--output", output, "--format", "json"}
		}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := writeCLIDesktopObservationProject(t)
			output := filepath.Join(dir, "qa-output")
			cmd := newQACmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(test.args(dir, output))

			err := cmd.Execute()
			require.Error(t, err)
			payload := decodeJSONMap(t, stdout.Bytes())
			assert.Equal(t, "qa_runtime_provider_required", payload["error"].(map[string]any)["code"])
			assert.NoDirExists(t, output)
		})
	}
}

func TestQARuntimeProvider_InvalidDuplicateAndConflictFailBeforeExecution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []string
		wantCode  string
	}{
		{name: "invalid", providers: []string{"automatic"}, wantCode: "qa_runtime_provider_invalid"},
		{name: "duplicate", providers: []string{"local", "local"}, wantCode: "qa_runtime_provider_conflict"},
		{name: "conflict", providers: []string{"local", "orca"}, wantCode: "qa_runtime_provider_conflict"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := writeCLIDesktopObservationProject(t)
			output := filepath.Join(dir, "qa-output")
			args := []string{"run", "--project-dir", dir, "--adapter", "desktop-accessibility-observe", "--lane", "desktop-native", "--output", output, "--format", "json"}
			for _, provider := range test.providers {
				args = append(args, "--runtime-provider", provider)
			}
			cmd := newQACmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(args)

			err := cmd.Execute()
			require.Error(t, err)
			payload := decodeJSONMap(t, stdout.Bytes())
			assert.Equal(t, test.wantCode, payload["error"].(map[string]any)["code"])
			assert.NoDirExists(t, output)
		})
	}
}

func TestQARuntimeProvider_NonObservationAdapterNeedsNoSelection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture.test\n"), 0o600))
	cmd := newQACmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"run", "--project-dir", dir, "--adapter", "go-test", "--dry-run", "--format", "json"})

	require.NoError(t, cmd.Execute())
	payload := decodeJSONMap(t, stdout.Bytes())
	assert.NotEqual(t, "qa_runtime_provider_required", payload["error"])
}

func TestQARuntimeProvider_ReconstructedCommandsPreserveSelection(t *testing.T) {
	t.Parallel()

	releaseCommand := releaseCommandString(qaReleaseOptions{
		Profile: "prelaunch", RuntimeProviders: []string{"orca"},
	}, true)
	assertRuntimeProviderFlagOnce(t, releaseCommand, "orca")

	fullOpts := qaFullOptions{ProjectDir: "desktop", RuntimeProviders: []string{"local"}}
	assertRuntimeProviderFlagOnce(t, qaFullCommandString(fullOpts, true), "local")
	next := qaFullNextCommands(fullOpts, qarelease.Plan{
		Profile: "prelaunch",
		JourneyPacks: []qarelease.JourneyPackRow{{
			Lane: "desktop-native", JourneyID: "desktop-accessibility-observe",
		}},
	}, qaFullDomainReadiness{Status: "ready"})
	require.NotEmpty(t, next)
	assertRuntimeProviderFlagOnce(t, next[0], "local")

	candidates := qaFullProjectCandidateCommands(fullOpts, []qaFullProjectCandidate{{RelPath: "desktop"}})
	require.NotEmpty(t, candidates)
	for _, command := range candidates {
		assertRuntimeProviderFlagOnce(t, command, "local")
	}
}

func writeCLIDesktopObservationProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".autopus", "qa", "journeys"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src-tauri"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src-tauri", "Cargo.toml"), []byte("[package]\nname='fixture'\n"), 0o600))
	yaml := strings.TrimSpace(`
id: desktop-accessibility-observe
title: Desktop accessibility observation
surface: desktop
lanes: [desktop-native]
adapter: {id: desktop-accessibility-observe}
command: {}
checks:
  - {id: semantic-landmarks, type: desktop_accessibility_semantic}
artifacts: []
desktop_observation:
  platform: macos
  operations: [capabilities, permissions, list_apps, list_windows, get_state]
  app_ref: autopus-desktop
  window_ref: main-window
  required_landmarks:
    - {role: AXApplication, name: Autopus, required_state: enabled}
    - {role: AXWindow, name: Autopus, required_state: focused}
source_refs:
  source_spec: SPEC-QAMESH-012
  acceptance_refs: [AC-QAMESH12-012]
pass_fail_authority: deterministic
`) + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".autopus", "qa", "journeys", "desktop-accessibility-observe.yaml"), []byte(yaml), 0o600))
	return dir
}
