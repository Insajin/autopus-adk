package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// RunMetricWithTimeout executes cmd with a deadline derived from cfg.ExperimentTimeout.
// If cfg.ExperimentTimeout is zero, no additional timeout is applied.
func RunMetricWithTimeout(cfg Config, cmd string) (MetricOutput, error) {
	ctx := context.Background()

	if cfg.ExperimentTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.ExperimentTimeout)
		defer cancel()
	}

	return RunMetric(ctx, cmd)
}

// RunMetricMedianWithTimeout runs cmd cfg.MetricRuns times with per-run timeout and returns median.
func RunMetricMedianWithTimeout(cfg Config, cmd string) (MetricOutput, error) {
	ctx := context.Background()

	if cfg.ExperimentTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.ExperimentTimeout)
		defer cancel()
	}

	runs := cfg.MetricRuns
	if runs <= 0 {
		runs = 1
	}

	return RunMetricMedian(ctx, cmd, runs)
}

// @AX:WARN [AUTO]: cmd is passed directly to sh -c without sanitization.
// @AX:REASON: Caller must ensure cmd originates from a trusted config (CLI flags or SPEC), never from
// user-supplied stdin or untrusted external sources. Arbitrary shell injection is possible otherwise.
// RunMetric executes cmd via shell and parses the first JSON object from stdout.
func RunMetric(ctx context.Context, cmd string) (MetricOutput, error) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	rawBytes, err := c.Output()
	if err != nil {
		return MetricOutput{}, fmt.Errorf("metric command failed: %w", err)
	}

	jsonStr, err := extractFirstJSON(string(rawBytes))
	if err != nil {
		return MetricOutput{}, fmt.Errorf("no JSON found in metric output: %w", err)
	}

	return parseMetricJSON(jsonStr)
}

// ExtractMetric retrieves a float64 value from a MetricOutput.
// If key is empty or not found in Metadata, returns the top-level Metric field.
func ExtractMetric(out MetricOutput, key string) (float64, error) {
	if key == "" {
		return out.Metric, nil
	}

	val, ok := out.Metadata[key]
	if !ok {
		return 0, fmt.Errorf("metric key %q not found in metadata", key)
	}

	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("metric key %q is not a float64 (got %T)", key, val)
	}

	return f, nil
}

// RunMetricMedian runs cmd runs times and returns the MetricOutput with the median Metric value.
func RunMetricMedian(ctx context.Context, cmd string, runs int) (MetricOutput, error) {
	if runs <= 0 {
		runs = 1
	}

	results := make([]MetricOutput, 0, runs)
	for i := 0; i < runs; i++ {
		out, err := RunMetric(ctx, cmd)
		if err != nil {
			return MetricOutput{}, fmt.Errorf("run %d/%d failed: %w", i+1, runs, err)
		}
		results = append(results, out)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Metric < results[j].Metric
	})

	mid := len(results) / 2
	if len(results)%2 == 0 {
		// Even: average the two middle values
		avg := (results[mid-1].Metric + results[mid].Metric) / 2.0
		median := results[mid]
		median.Metric = avg
		return median, nil
	}

	return results[mid], nil
}

// @AX:NOTE [AUTO]: Uses brace-depth + escape-aware string tracking to handle mixed stdout.
// Simple strings.Index("{") would fail on nested JSON or embedded JSON within log lines.
// extractFirstJSON finds the first complete JSON object in raw using brace matching.
func extractFirstJSON(raw string) (string, error) {
	start := strings.Index(raw, "{")
	if start == -1 {
		return "", fmt.Errorf("no JSON object found in output")
	}

	depth := 0
	inStr := false
	escape := false

	for i := start; i < len(raw); i++ {
		ch := raw[i]

		if escape {
			escape = false
			continue
		}

		if ch == '\\' && inStr {
			escape = true
			continue
		}

		if ch == '"' {
			inStr = !inStr
			continue
		}

		if inStr {
			continue
		}

		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				candidate := raw[start : i+1]
				// Validate it is parseable JSON
				var check map[string]any
				if err := json.Unmarshal([]byte(candidate), &check); err != nil {
					return "", fmt.Errorf("extracted text is not valid JSON: %w", err)
				}
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("unclosed JSON object in output")
}

// parseMetricJSON decodes a JSON string into MetricOutput.
func parseMetricJSON(jsonStr string) (MetricOutput, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return MetricOutput{}, fmt.Errorf("invalid metric JSON: %w", err)
	}

	out := MetricOutput{
		Metadata: make(map[string]any),
	}

	if v, ok := raw["metric"]; ok {
		if f, ok := v.(float64); ok {
			out.Metric = f
		}
	}

	if v, ok := raw["unit"]; ok {
		if s, ok := v.(string); ok {
			out.Unit = s
		}
	}

	// Everything else goes into Metadata
	for k, v := range raw {
		if k != "metric" && k != "unit" {
			out.Metadata[k] = v
		}
	}

	return out, nil
}
