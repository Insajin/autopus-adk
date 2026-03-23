package experiment

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupRepo is a helper for coverage tests to avoid duplication.
func setupRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "exp-cov-*")
	require.NoError(t, err)

	for _, args := range [][]string{
		{"git", "-c", "gc.auto=0", "init"},
		{"git", "-c", "gc.auto=0", "config", "user.email", "t@t.com"},
		{"git", "-c", "gc.auto=0", "config", "user.name", "T"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}

	f := filepath.Join(dir, "init.txt")
	require.NoError(t, os.WriteFile(f, []byte("init\n"), 0644))
	for _, args := range [][]string{
		{"git", "-c", "gc.auto=0", "add", "."},
		{"git", "-c", "gc.auto=0", "commit", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}

	return dir, func() { _ = os.RemoveAll(dir) }
}

// --- circuit.go coverage ---

func TestCircuitBreaker_ConsecutiveNoProgress(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(5)
	assert.Equal(t, 0, cb.ConsecutiveNoProgress())

	cb.Record(false)
	cb.Record(false)
	assert.Equal(t, 2, cb.ConsecutiveNoProgress())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	t.Parallel()

	cb := NewCircuitBreaker(3)
	cb.Record(false)
	cb.Record(false)
	assert.Equal(t, 2, cb.ConsecutiveNoProgress())

	cb.Reset()
	assert.Equal(t, 0, cb.ConsecutiveNoProgress())
	assert.False(t, cb.IsTripped())
}

// --- git.go CheckScope coverage ---

func TestCheckScope_AllInScope(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("scope-ok"))

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	base := string(out[:40])

	// Add a file within scope
	require.NoError(t, os.WriteFile(filepath.Join(dir, "allowed.go"), []byte("package main\n"), 0644))
	_, err = g.CommitExperiment(1, "add allowed")
	require.NoError(t, err)

	ok, violations, err := g.CheckScope(base, []string{"allowed.go"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, violations)
}

func TestCheckScope_OutOfScope(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("scope-viol"))

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	base := string(out[:40])

	// Add file outside scope
	require.NoError(t, os.WriteFile(filepath.Join(dir, "outside.go"), []byte("package main\n"), 0644))
	_, err = g.CommitExperiment(1, "add outside")
	require.NoError(t, err)

	ok, violations, err := g.CheckScope(base, []string{"allowed.go"})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, violations, "outside.go")
}

func TestCheckScope_EmptyAllowedPaths(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("scope-empty"))

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	base := string(out[:40])

	require.NoError(t, os.WriteFile(filepath.Join(dir, "any.go"), []byte("package main\n"), 0644))
	_, err = g.CommitExperiment(1, "add any")
	require.NoError(t, err)

	// Empty allowed paths means nothing is allowed
	ok, violations, err := g.CheckScope(base, []string{})
	require.NoError(t, err)
	assert.False(t, ok)
	assert.NotEmpty(t, violations)
}

func TestCheckScope_NoChanges(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupRepo(t)
	defer cleanup()

	g := NewGit(dir)
	require.NoError(t, g.CreateExperimentBranch("scope-nochange"))

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	base := string(out[:40])

	// No changes since base — scope check should pass with no violations
	ok, violations, err := g.CheckScope(base, []string{"anything.go"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Empty(t, violations)
}

// --- metric.go RunMetricWithTimeout / RunMetricMedianWithTimeout coverage ---

func TestRunMetricWithTimeout_Success(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ExperimentTimeout = 5 * time.Second

	out, err := RunMetricWithTimeout(cfg, `echo '{"metric": 3.14}'`)
	require.NoError(t, err)
	assert.Equal(t, 3.14, out.Metric)
}

func TestRunMetricWithTimeout_ZeroTimeout(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ExperimentTimeout = 0 // no timeout

	out, err := RunMetricWithTimeout(cfg, `echo '{"metric": 1.0}'`)
	require.NoError(t, err)
	assert.Equal(t, 1.0, out.Metric)
}

func TestRunMetricWithTimeout_Cancelled(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ExperimentTimeout = 1 * time.Millisecond // extremely short

	// sleep 1 should timeout
	_, err := RunMetricWithTimeout(cfg, "sleep 1 && echo '{\"metric\": 1.0}'")
	assert.Error(t, err)
}

func TestRunMetricMedianWithTimeout_Success(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.ExperimentTimeout = 10 * time.Second
	cfg.MetricRuns = 3

	out, err := RunMetricMedianWithTimeout(cfg, `echo '{"metric": 2.0}'`)
	require.NoError(t, err)
	assert.Equal(t, 2.0, out.Metric)
}

func TestRunMetricMedianWithTimeout_ZeroRuns(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MetricRuns = 0 // should default to 1

	out, err := RunMetricMedianWithTimeout(cfg, `echo '{"metric": 5.0}'`)
	require.NoError(t, err)
	assert.Equal(t, 5.0, out.Metric)
}

// --- metric.go extractFirstJSON edge cases ---

func TestExtractFirstJSON_MultipleObjects(t *testing.T) {
	t.Parallel()

	// Should return the first JSON object only
	raw := `{"metric": 1.0} {"metric": 2.0}`
	got, err := extractFirstJSON(raw)
	require.NoError(t, err)
	assert.Contains(t, got, "1.0")
}

func TestRunMetric_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := RunMetric(ctx, "sleep 1 && echo '{\"metric\": 1.0}'")
	assert.Error(t, err)
}

// --- git.go sessionFromBranch non-experiment branch ---

func TestSessionFromBranch_NonExperimentBranch(t *testing.T) {
	t.Parallel()

	dir, cleanup := setupRepo(t)
	defer cleanup()

	g := NewGit(dir)

	// On main/master branch, sessionFromBranch should return "unknown"
	session := g.sessionFromBranch()
	assert.Equal(t, "unknown", session)
}

// --- simplicity.go default branch coverage ---

func TestCalculateSimplicity_DefaultDirection(t *testing.T) {
	t.Parallel()

	// Direction(99) is not Minimize or Maximize, hits default branch
	score := CalculateSimplicity(100.0, 90.0, 5, 0, Direction(99))
	// default branch uses baseline - current (same as Minimize)
	assert.Greater(t, score, 0.0)
}
