package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type workflowContextProductIdentityDriverSpy struct {
	*recordingManagedWorkflowContextDriver
	verifyCalls   int
	verifyVersion string
	verifyErr     error
}

func (driver *workflowContextProductIdentityDriverSpy) verifyInstalledIdentity(
	_ context.Context, version string,
) error {
	driver.verifyCalls++
	driver.verifyVersion = version
	return driver.verifyErr
}

func TestWorkflowContextProductSession_BindsExactPromptsAndGrammar(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*WorkflowContextProductSessionInput)
	}{
		{name: "command mismatch", mutate: func(input *WorkflowContextProductSessionInput) {
			input.OriginalTask = "/auto fix SPEC-OMP-004 --auto"
		}},
		{name: "SPEC mismatch", mutate: func(input *WorkflowContextProductSessionInput) {
			input.OriginalTask = "/auto go SPEC-OTHER --auto"
		}},
		{name: "empty second prompt", mutate: func(input *WorkflowContextProductSessionInput) {
			input.DecisionDelta = ""
		}},
		{name: "later SPEC token bypass", mutate: func(input *WorkflowContextProductSessionInput) {
			input.OriginalTask = "/auto go OTHER --note SPEC-OMP-004"
		}},
		{name: "alias later SPEC token bypass", mutate: func(input *WorkflowContextProductSessionInput) {
			input.OriginalTask = "/auto-go OTHER --note SPEC-OMP-004"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, runtime, _, factory := newWorkflowContextProductFixture(t)
			test.mutate(&input)

			_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

			require.Error(t, err)
			assert.Zero(t, factory.calls)
		})
	}
}

func TestWorkflowContextProductSession_RequiresCanonicalSpecDirectory(t *testing.T) {
	t.Run("absolute path normalizes to relative delivery authority", func(t *testing.T) {
		input, runtime, _, factory := newWorkflowContextProductFixture(t)
		input.SpecDir = filepath.Join(input.ProjectDir, filepath.FromSlash(input.SpecDir))

		receipt, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

		require.NoError(t, err)
		assert.Equal(t, WorkflowContextOutcomeAdmitted, receipt.Outcome)
		assert.Equal(t, 1, factory.calls)
	})

	t.Run("basename mismatch", func(t *testing.T) {
		input, runtime, _, factory := newWorkflowContextProductFixture(t)
		input.SpecID = "SPEC-OTHER"
		input.OriginalTask = "/auto go SPEC-OTHER --auto"

		_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

		require.Error(t, err)
		assert.Zero(t, factory.calls)
	})

	t.Run("outside project", func(t *testing.T) {
		input, runtime, _, factory := newWorkflowContextProductFixture(t)
		outside := filepath.Join(t.TempDir(), input.SpecID)
		require.NoError(t, os.MkdirAll(outside, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(outside, "spec.md"), []byte("outside"), 0o600))
		input.SpecDir = outside

		_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

		require.Error(t, err)
		assert.Zero(t, factory.calls)
	})

	t.Run("symlink spec document", func(t *testing.T) {
		input, runtime, _, factory := newWorkflowContextProductFixture(t)
		spec := filepath.Join(input.ProjectDir, filepath.FromSlash(input.SpecDir), "spec.md")
		target := filepath.Join(t.TempDir(), "spec.md")
		require.NoError(t, os.WriteFile(target, []byte("linked"), 0o600))
		require.NoError(t, os.Remove(spec))
		require.NoError(t, os.Symlink(target, spec))

		_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

		require.Error(t, err)
		assert.Zero(t, factory.calls)
	})

	t.Run("symlink spec directory", func(t *testing.T) {
		input, runtime, _, factory := newWorkflowContextProductFixture(t)
		spec := filepath.Join(input.ProjectDir, filepath.FromSlash(input.SpecDir))
		target := spec + "-target"
		require.NoError(t, os.Rename(spec, target))
		require.NoError(t, os.Symlink(target, spec))

		_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

		require.Error(t, err)
		assert.Zero(t, factory.calls)
	})
}

func TestWorkflowContextProductSession_VerifiesInstalledIdentityBeforeRun(t *testing.T) {
	input, runtime, driver, factory := newWorkflowContextProductFixture(t)
	verified := factory.driver.(*workflowContextProductIdentityDriverSpy)
	verified.verifyErr = errors.New("identity drift")

	_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

	require.ErrorContains(t, err, "identity drift")
	assert.Equal(t, 1, verified.verifyCalls)
	assert.Equal(t, runtime.Capabilities.Version, verified.verifyVersion)
	assert.Zero(t, driver.runCalls)
	assert.Equal(t, 1, driver.cleanupCalls)
}

func TestWorkflowContextProductSession_RejectsUnverifiedDriverAndCapabilities(t *testing.T) {
	t.Run("driver has no verifier", func(t *testing.T) {
		input, runtime, driver, factory := newWorkflowContextProductFixture(t)
		factory.driver = driver

		_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

		require.ErrorContains(t, err, "identity verifier")
		assert.Zero(t, driver.runCalls)
		assert.Equal(t, 1, driver.cleanupCalls)
	})

	for _, test := range []struct {
		name   string
		mutate func(*WorkflowContextCapabilities)
	}{
		{name: "unobserved version", mutate: func(capabilities *WorkflowContextCapabilities) {
			capabilities.Version = "OMP version unknown"
		}},
		{name: "provider credential authority unproved", mutate: func(capabilities *WorkflowContextCapabilities) {
			capabilities.ProviderCredentialAuthority = false
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, runtime, _, factory := newWorkflowContextProductFixture(t)
			test.mutate(&runtime.Capabilities)

			_, err := RunWorkflowContextProductSession(context.Background(), input, runtime)

			require.Error(t, err)
			assert.Zero(t, factory.calls)
		})
	}
}
