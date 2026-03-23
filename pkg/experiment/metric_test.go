package experiment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFirstJSON_ValidJSON(t *testing.T) {
	t.Parallel()

	raw := `some prefix text {"metric": 1.5, "unit": "ms"} trailing text`
	got, err := extractFirstJSON(raw)
	require.NoError(t, err)
	assert.JSONEq(t, `{"metric": 1.5, "unit": "ms"}`, got)
}

func TestExtractFirstJSON_MixedOutput(t *testing.T) {
	t.Parallel()

	raw := "running benchmark...\nwarm up done\n{\"metric\": 42.0}\nDone."
	got, err := extractFirstJSON(raw)
	require.NoError(t, err)
	assert.Contains(t, got, "42.0")
}

func TestExtractFirstJSON_NestedJSON(t *testing.T) {
	t.Parallel()

	raw := `{"metric": 1.5, "metadata": {"p50": 1.0, "p99": 2.5}}`
	got, err := extractFirstJSON(raw)
	require.NoError(t, err)
	assert.JSONEq(t, raw, got)
}

func TestExtractFirstJSON_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
	}{
		{"empty string", ""},
		{"no json", "just plain text"},
		{"unclosed brace", "{unclosed"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := extractFirstJSON(tc.raw)
			assert.Error(t, err)
		})
	}
}

func TestRunMetric_EchoJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Use echo to simulate a command that outputs JSON
	out, err := RunMetric(ctx, `echo '{"metric": 1.5, "unit": "ms"}'`)
	require.NoError(t, err)
	assert.Equal(t, 1.5, out.Metric)
	assert.Equal(t, "ms", out.Unit)
}

func TestRunMetric_FailsOnBadCmd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := RunMetric(ctx, "this-command-does-not-exist-xyzzy")
	assert.Error(t, err)
}

func TestRunMetric_FailsOnNoJSON(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := RunMetric(ctx, "echo 'no json here'")
	assert.Error(t, err)
}

func TestExtractMetric_DirectMetric(t *testing.T) {
	t.Parallel()

	out := MetricOutput{
		Metric: 3.14,
		Unit:   "s",
	}
	val, err := ExtractMetric(out, "")
	require.NoError(t, err)
	assert.Equal(t, 3.14, val)
}

func TestExtractMetric_FromMetadata(t *testing.T) {
	t.Parallel()

	out := MetricOutput{
		Metric: 0,
		Metadata: map[string]any{
			"p99": float64(9.9),
		},
	}
	val, err := ExtractMetric(out, "p99")
	require.NoError(t, err)
	assert.Equal(t, 9.9, val)
}

func TestExtractMetric_MissingKey(t *testing.T) {
	t.Parallel()

	out := MetricOutput{
		Metric:   0,
		Metadata: map[string]any{},
	}
	_, err := ExtractMetric(out, "nonexistent")
	assert.Error(t, err)
}

func TestRunMetricMedian_OddRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// 3 identical runs -> median equals the single value
	out, err := RunMetricMedian(ctx, `echo '{"metric": 5.0}'`, 3)
	require.NoError(t, err)
	assert.Equal(t, 5.0, out.Metric)
}

func TestRunMetricMedian_EvenRuns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Even number of runs — median of 2 identical values
	out, err := RunMetricMedian(ctx, `echo '{"metric": 2.0}'`, 2)
	require.NoError(t, err)
	assert.Equal(t, 2.0, out.Metric)
}

func TestRunMetricMedian_SingleRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	out, err := RunMetricMedian(ctx, `echo '{"metric": 7.5}'`, 1)
	require.NoError(t, err)
	assert.Equal(t, 7.5, out.Metric)
}
