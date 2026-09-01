package cli

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCanaryBuildTargets_EmptyForProjectWithoutStack proves a directory that
// declares no stack yields no build target rather than a FAIL for a directory
// that never existed.
func TestCanaryBuildTargets_EmptyForProjectWithoutStack(t *testing.T) {
	t.Parallel()

	targets, skips := canaryBuildTargets(t.TempDir())

	assert.Empty(t, targets)
	assert.Empty(t, skips)
}

// TestCanaryBuildTargets_DerivedFromProjectRoot proves detection reads the
// project under test, not a hardcoded Autopus sibling layout.
func TestCanaryBuildTargets_DerivedFromProjectRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test\n"), 0o644))

	targets, skips := canaryBuildTargets(dir)

	require.Len(t, targets, 1)
	assert.Equal(t, "build:.:go", targets[0].ID)
	assert.Equal(t, dir, targets[0].Dir)
	assert.Equal(t, []string{"go", "build", "./..."}, targets[0].Args)
	assert.Empty(t, skips)
}

// TestErrOrDefault_UseErrWhenPresent returns the existing error unchanged.
func TestErrOrDefault_UseErrWhenPresent(t *testing.T) {
	t.Parallel()

	original := errors.New("original")
	got := errOrDefault(original, "fallback message")
	assert.Equal(t, original, got)
}

// TestErrOrDefault_NilErrReturnsFallback wraps the message in a new error.
func TestErrOrDefault_NilErrReturnsFallback(t *testing.T) {
	t.Parallel()

	got := errOrDefault(nil, "fallback message")
	require.Error(t, got)
	assert.Equal(t, "fallback message", got.Error())
}

// TestPrintCanaryText_RendersVerdictAndSummary verifies text output format.
func TestPrintCanaryText_RendersVerdictAndSummary(t *testing.T) {
	t.Parallel()

	cmd := newAgentCmd() // any cobra.Command with a stdout
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	result := canaryResult{
		Verdict: "PASS",
		Summary: map[string]string{
			"build": "PASS",
			"e2e":   "SKIP",
		},
	}
	printCanaryText(cmd, result)

	out := buf.String()
	assert.Contains(t, out, "canary PASS")
	assert.Contains(t, out, "build")
	assert.Contains(t, out, "PASS")
}

// TestWriteCanaryLatest_PersistsJSON verifies the file is written as parseable JSON.
func TestWriteCanaryLatest_PersistsJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := canaryResult{
		Verdict: "PASS",
		Summary: map[string]string{"build": "PASS"},
	}
	require.NoError(t, writeCanaryLatest(dir, result))

	data, err := os.ReadFile(filepath.Join(dir, ".autopus", "canary", "latest.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"verdict"`)
	assert.Contains(t, string(data), "PASS")
}

// TestRunCanaryExternal_EmptyCommand returns FAIL with descriptive detail.
func TestRunCanaryExternal_EmptyCommand(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	result := runCanaryExternal(ctx, "H-test", "empty command test", t.TempDir())
	assert.Equal(t, "FAIL", result.Status)
	assert.Contains(t, result.Detail, "empty command")
}

func TestRunCanaryEndpointChecks_AcceptsProtectedMetrics(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/metrics":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	result := canaryResult{}
	status := runCanaryEndpointChecks(t.Context(), server.URL, &result)

	assert.Equal(t, "PASS", status)
	require.Len(t, result.Targets, 2)
	assert.Equal(t, "PASS", result.Targets[0].Status)
	assert.Contains(t, result.Targets[0].Detail, "HTTP 200")
	assert.Equal(t, "PASS", result.Targets[1].Status)
	assert.Contains(t, result.Targets[1].Detail, "HTTP 401")
	assert.Contains(t, result.Targets[1].Detail, "protected")
}

func TestRunCanaryEndpointChecks_ReportsIndependentFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusInternalServerError)
		case "/metrics":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	result := canaryResult{}
	status := runCanaryEndpointChecks(t.Context(), server.URL, &result)

	assert.Equal(t, "FAIL", status)
	require.Len(t, result.Targets, 2)
	assert.Equal(t, "FAIL", result.Targets[0].Status)
	assert.Contains(t, result.Targets[0].Detail, "HTTP 500")
	assert.Equal(t, "PASS", result.Targets[1].Status)
	assert.Contains(t, result.Targets[1].Detail, "HTTP 200")
}

func TestRunCanaryEndpointChecks_RejectsMetricsErrors(t *testing.T) {
	t.Parallel()

	for _, code := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/health" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(code)
			}))
			t.Cleanup(server.Close)

			result := canaryResult{}
			status := runCanaryEndpointChecks(t.Context(), server.URL, &result)

			assert.Equal(t, "FAIL", status)
			require.Len(t, result.Targets, 2)
			assert.Equal(t, "FAIL", result.Targets[1].Status)
			assert.Contains(t, result.Targets[1].Detail, fmt.Sprintf("HTTP %d", code))
		})
	}
}

func TestRunCanaryEndpointChecks_LimitsUnauthorizedExceptionToMetrics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		healthCode   int
		metricsCode  int
		failedTarget int
	}{
		{name: "health unauthorized", healthCode: http.StatusUnauthorized, metricsCode: http.StatusUnauthorized, failedTarget: 0},
		{name: "metrics forbidden", healthCode: http.StatusOK, metricsCode: http.StatusForbidden, failedTarget: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/health" {
					w.WriteHeader(tc.healthCode)
					return
				}
				w.WriteHeader(tc.metricsCode)
			}))
			t.Cleanup(server.Close)

			result := canaryResult{}
			status := runCanaryEndpointChecks(t.Context(), server.URL, &result)

			assert.Equal(t, "FAIL", status)
			require.Len(t, result.Targets, 2)
			assert.Equal(t, "FAIL", result.Targets[tc.failedTarget].Status)
		})
	}
}

func TestRunCanaryEndpointChecks_RejectsTransportErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL
	server.Close()

	result := canaryResult{}
	status := runCanaryEndpointChecks(t.Context(), baseURL, &result)

	assert.Equal(t, "FAIL", status)
	require.Len(t, result.Targets, 2)
	for _, target := range result.Targets {
		assert.Equal(t, "FAIL", target.Status)
		assert.NotEmpty(t, target.Detail)
	}
}
