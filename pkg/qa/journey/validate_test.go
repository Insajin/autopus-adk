package journey

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAcceptsSafeCustomCommand(t *testing.T) {
	t.Parallel()

	pack := Pack{
		ID:      "custom-safe",
		Lanes:   []string{"fast"},
		Surface: "custom",
		Adapter: AdapterRef{ID: "custom-command"},
		Command: Command{Argv: []string{"go", "test", "./..."}, CWD: ".", Timeout: "60s"},
		Checks:  []Check{{ID: "unit", Type: "unit_test"}},
		Artifacts: []Artifact{
			{Root: ".autopus/qa/runs"},
		},
	}

	require.NoError(t, Validate(pack, t.TempDir()))
}

func TestLoadDirLoadsAndFiltersLanes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := []byte(`id: cli-smoke
title: CLI smoke
surface: cli
lanes: [fast]
adapter:
  id: go-test
command:
  run: go test ./...
  cwd: .
  timeout: 60s
checks:
  - id: unit
    type: unit_test
artifacts:
  - root: .autopus/qa/runs
source_refs:
  source_spec: SPEC-QAMESH-002
  acceptance_refs: [AC-QAMESH2-001]
`)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "cli-smoke.yaml"), body, 0o644))

	packs, err := LoadDir(dir)
	require.NoError(t, err)
	require.Len(t, packs, 1)
	assert.Equal(t, "configured", packs[0].Source)
	assert.True(t, HasLane(packs[0], "fast"))
	assert.False(t, HasLane(packs[0], "release"))
}

func TestValidateRejectsUnsafeJourneyCommand(t *testing.T) {
	t.Parallel()

	pack := Pack{
		ID:      "unsafe",
		Lanes:   []string{"fast"},
		Surface: "custom",
		Adapter: AdapterRef{ID: "custom-command"},
		Command: Command{Argv: []string{"sh", "-c", "rm -rf /"}, CWD: "../outside", Timeout: "0"},
		Checks:  []Check{{ID: "unit", Type: "unit_test"}},
	}

	err := Validate(pack, t.TempDir())
	require.Error(t, err)
	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Contains(t, validationErr.Code, "qa_journey_")
}

func TestValidateCompiledCommandUsesCompilerErrorCodes(t *testing.T) {
	t.Parallel()

	err := ValidateCompiledCommand("custom-command", Command{
		Argv:    []string{"sh", "-c", "rm -rf /"},
		CWD:     ".",
		Timeout: "60s",
	}, nil, t.TempDir())
	require.Error(t, err)

	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "qa_compiler_command_unsafe", validationErr.Code)
}

func TestValidateCommandRejectsSpecificUnsafeInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  Command
		arts []Artifact
		want string
	}{
		{name: "timeout", cmd: Command{Run: "go test ./...", CWD: ".", Timeout: "0"}, want: "qa_journey_timeout_invalid"},
		{name: "cwd", cmd: Command{Run: "go test ./...", CWD: "../outside", Timeout: "60s"}, want: "qa_journey_cwd_outside_project"},
		{name: "artifact", cmd: Command{Run: "go test ./...", CWD: ".", Timeout: "60s"}, arts: []Artifact{{Path: "/tmp/raw.log"}}, want: "qa_journey_artifact_path_invalid"},
		{name: "env", cmd: Command{Run: "go test ./...", CWD: ".", Timeout: "60s", EnvAllowlist: []string{"SECRET=value"}}, want: "qa_journey_env_not_allowlisted"},
		{name: "shape", cmd: Command{Run: "npm test", CWD: ".", Timeout: "60s"}, want: "qa_journey_command_unsafe"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCommand("go-test", tt.cmd, tt.arts, t.TempDir(), "qa_journey")
			require.Error(t, err)
			var validationErr *ValidationError
			require.True(t, errors.As(err, &validationErr))
			assert.Equal(t, tt.want, validationErr.Code)
		})
	}
}

func TestValidateCommandAcceptsBuiltInRunnerShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		adapterID string
		run       string
	}{
		{adapterID: "go-test", run: "go test ./..."},
		{adapterID: "node-script", run: "npm test"},
		{adapterID: "node-script", run: "pnpm test"},
		{adapterID: "pytest", run: "pytest"},
		{adapterID: "pytest", run: "python -m pytest"},
		{adapterID: "cargo-test", run: "cargo test"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.adapterID+" "+tt.run, func(t *testing.T) {
			t.Parallel()
			err := ValidateCommand(tt.adapterID, Command{Run: tt.run, CWD: ".", Timeout: "60s"}, nil, t.TempDir(), "qa_journey")
			require.NoError(t, err)
		})
	}
}

func TestValidateRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	base := Pack{
		ID:      "x",
		Lanes:   []string{"fast"},
		Adapter: AdapterRef{ID: "go-test"},
		Command: Command{Run: "go test ./...", CWD: ".", Timeout: "60s"},
		Checks:  []Check{{ID: "unit", Type: "unit_test"}},
	}
	tests := []struct {
		name   string
		mutate func(*Pack)
	}{
		{name: "id", mutate: func(pack *Pack) { pack.ID = "" }},
		{name: "adapter", mutate: func(pack *Pack) { pack.Adapter.ID = "" }},
		{name: "lanes", mutate: func(pack *Pack) { pack.Lanes = nil }},
		{name: "checks", mutate: func(pack *Pack) { pack.Checks = nil }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pack := base
			tt.mutate(&pack)
			require.Error(t, Validate(pack, t.TempDir()))
		})
	}
}

func TestValidateCommandRejectsAdditionalEdges(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateCommand("go-test", Command{Run: "go test ./...", CWD: "", Timeout: ""}, nil, t.TempDir(), "qa_journey"))
	assert.Error(t, ValidateCommand("custom-command", Command{Argv: []string{""}, CWD: ".", Timeout: "60s"}, nil, t.TempDir(), "qa_journey"))
	assert.Error(t, ValidateCommand("custom-command", Command{Argv: []string{"bash", "-lc", "echo hi"}, CWD: ".", Timeout: "60s"}, nil, t.TempDir(), "qa_journey"))
	assert.Error(t, ValidateCommand("go-test", Command{Run: "go test ./...", CWD: "/tmp", Timeout: "60s"}, nil, t.TempDir(), "qa_journey"))
	assert.Error(t, ValidateCommand("go-test", Command{Run: "go test ./...", CWD: ".", Timeout: "not-a-duration"}, nil, t.TempDir(), "qa_journey"))
	assert.Error(t, ValidateCommand("go-test", Command{Run: "go test ./...", CWD: ".", Timeout: "31m"}, nil, t.TempDir(), "qa_journey"))
	assert.Error(t, ValidateCommand("go-test", Command{Run: "go test ./...", CWD: ".", Timeout: "60s"}, []Artifact{{Root: "../outside"}}, t.TempDir(), "qa_journey"))
	assert.Equal(t, "x", (&ValidationError{Message: "x"}).Error())
}

func TestValidateCommandRejectsAdapterArgvBypassAndUnsafeCanary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.Error(t, ValidateCommand("go-test", Command{Argv: []string{"sh", "-c", "go test ./..."}, CWD: ".", Timeout: "60s"}, nil, dir, "qa_journey"))
	assert.Error(t, ValidateCommand("custom-command", Command{Argv: []string{"env", "sh", "-c", "echo hi"}, CWD: ".", Timeout: "60s"}, nil, dir, "qa_journey"))
	assert.Error(t, ValidateCommand("canary-template", Command{Run: "rm -rf .", CWD: ".", Timeout: "60s"}, nil, dir, "qa_journey"))
	assert.NoError(t, ValidateCommand("canary-template", Command{Run: "auto canary", CWD: ".", Timeout: "60s"}, nil, dir, "qa_journey"))
}

func TestValidateCommandRejectsSymlinkCWDEscape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "outside-link")))

	err := ValidateCommand("go-test", Command{Run: "go test ./...", CWD: "outside-link", Timeout: "60s"}, nil, dir, "qa_journey")

	require.Error(t, err)
	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "qa_journey_cwd_outside_project", validationErr.Code)
}
