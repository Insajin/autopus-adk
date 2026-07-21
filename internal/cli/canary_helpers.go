package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type canaryCommandTarget struct {
	ID      string
	Dir     string
	Command string
	Args    []string
}

func canaryBuildTargets(projectDir string) []canaryCommandTarget {
	return []canaryCommandTarget{
		{"H1", filepath.Join(projectDir, "autopus-adk"), "go build -o /tmp/autopus-canary-auto ./cmd/auto/", []string{"go", "build", "-o", "/tmp/autopus-canary-auto", "./cmd/auto/"}},
		{"H2", filepath.Join(projectDir, "Autopus", "backend"), "go build -o /tmp/autopus-canary-server ./cmd/server/", []string{"go", "build", "-o", "/tmp/autopus-canary-server", "./cmd/server/"}},
		{"H3", filepath.Join(projectDir, "Autopus", "backend"), "go build -o /tmp/autopus-canary-worker ./cmd/worker/", []string{"go", "build", "-o", "/tmp/autopus-canary-worker", "./cmd/worker/"}},
		{"H4", filepath.Join(projectDir, "Autopus", "frontend"), "npm run build", []string{"npm", "run", "build"}},
		{"H5a", filepath.Join(projectDir, "autopus-desktop"), "npm run build", []string{"npm", "run", "build"}},
		{"H5b", filepath.Join(projectDir, "autopus-desktop"), "cargo check --manifest-path src-tauri/Cargo.toml", []string{"cargo", "check", "--manifest-path", "src-tauri/Cargo.toml"}},
	}
}

func runCanaryExternal(ctx context.Context, id, display, dir string, argv ...string) canaryTargetResult {
	if len(argv) == 0 {
		return canaryTargetResult{ID: id, Command: display, Status: "FAIL", Detail: "empty command"}
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	c := exec.CommandContext(timeoutCtx, argv[0], argv[1:]...) //nolint:gosec
	c.Dir = dir
	output, err := c.CombinedOutput()
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return canaryTargetResult{ID: id, Command: display, Status: "FAIL", Detail: "timed out"}
	}
	if err != nil {
		return canaryTargetResult{ID: id, Command: display, Status: "FAIL", Detail: strings.TrimSpace(string(output))}
	}
	return canaryTargetResult{ID: id, Command: display, Status: "PASS"}
}

func runCanaryEndpointChecks(ctx context.Context, baseURL string, result *canaryResult) string {
	status := "PASS"
	endpoints := []struct {
		path               string
		acceptUnauthorized bool
	}{
		{path: "/health"},
		{path: "/metrics", acceptUnauthorized: true},
	}
	for _, endpoint := range endpoints {
		code, err := canaryHTTPStatus(ctx, strings.TrimRight(baseURL, "/")+endpoint.path)
		successResponse := code >= http.StatusOK && code < http.StatusBadRequest
		protectedResponse := endpoint.acceptUnauthorized && code == http.StatusUnauthorized
		passed := err == nil && (successResponse || protectedResponse)
		targetStatus := "PASS"
		detail := fmt.Sprintf("HTTP %d", code)
		if protectedResponse {
			detail += " (protected endpoint)"
		}
		if err != nil {
			detail = err.Error()
		}
		if !passed {
			status = "FAIL"
			targetStatus = "FAIL"
		}
		result.Targets = append(result.Targets, canaryTargetResult{
			ID:     "endpoint" + endpoint.path,
			Status: targetStatus,
			Detail: detail,
		})
	}
	return status
}

func canaryHTTPStatus(ctx context.Context, url string) (int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func writeCanaryLatest(projectDir string, result canaryResult) error {
	dir := filepath.Join(projectDir, ".autopus", "canary")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "latest.json"), append(data, '\n'), 0o644)
}

func canarySummary(result canaryResult) map[string]string {
	return map[string]string{
		"build":    result.Build,
		"e2e":      result.E2E,
		"doctor":   result.Doctor,
		"endpoint": result.Endpoint,
		"browser":  result.Browser,
	}
}

func canaryChecks(result canaryResult) []jsonCheck {
	checks := make([]jsonCheck, 0, len(result.Summary))
	for k, v := range result.Summary {
		checks = append(checks, jsonCheck{ID: "canary." + k, Status: strings.ToLower(v), Detail: k + "=" + v})
	}
	return checks
}

func printCanaryText(cmd *cobra.Command, result canaryResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "canary %s\n", result.Verdict)
	for key, value := range result.Summary {
		fmt.Fprintf(out, "%s: %s\n", key, value)
	}
}

func errOrDefault(err error, message string) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
