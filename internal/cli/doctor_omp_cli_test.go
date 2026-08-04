package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	ompDoctorSecretSentinel = "sk-omp-doctor-secret"
	ompDoctorRawPayload     = "RAW-OMP-PROVIDER-PAYLOAD"
)

func TestOMP002_S10_DoctorCLIProjectsStableOMPChecksInTextAndJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hermetic fake omp uses a POSIX shell script")
	}

	root := t.TempDir()
	cfg := config.DefaultFullConfig("omp-doctor-cli")
	cfg.Platforms = []string{"omp"}
	require.NoError(t, config.Save(root, cfg))
	_, err := omp.NewWithRoot(root).Generate(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, []string{"opus", "sonnet"}, generatedOMPDoctorSelectors(t, root))

	logPath := installHermeticOMPDoctorCLI(t)
	healthyText, healthyEnvelope := runOMPDoctorEntrypoints(t, root)
	healthyChecks := ompPlatformDoctorChecks(healthyEnvelope.Checks)
	require.NotEmpty(t, healthyChecks)
	assert.Empty(t, failingOMPDoctorChecks(healthyChecks),
		"healthy behavioral fixture must produce a 10/10 OMP readiness receipt")
	assert.Equal(t, jsonStatusWarn, healthyEnvelope.Status,
		"missing non-OMP dependencies may warn without changing OMP check outcomes")
	assert.NotEmpty(t, failingNonOMPDoctorChecks(healthyEnvelope.Checks),
		"the fixture deliberately keeps unrelated dependency warnings separate")
	assertOMPDoctorProjectionIsRedacted(t, root, healthyText, healthyChecks)

	configPath := filepath.Join(root, ".omp", "config.yml")
	configured, err := os.ReadFile(configPath)
	require.NoError(t, err)
	broken := strings.Replace(string(configured), ".agents/skills", "wrong/path", 1)
	require.NotEqual(t, string(configured), broken)
	require.NoError(t, os.WriteFile(configPath, []byte(broken), 0o600))

	failedText, failedEnvelope := runOMPDoctorEntrypoints(t, root)
	validationChecks := ompValidationDoctorChecks(failedEnvelope.Checks)
	require.Len(t, validationChecks, 1)
	check := validationChecks[0]
	assert.True(t, strings.HasPrefix(check.ID, "doctor.platform.omp.validation."))
	assert.Equal(t, "error", check.Severity)
	assert.Equal(t, "fail", check.Status)
	assert.Equal(t, "skills.customDirectories expected=[.agents/skills] got=[wrong/path]", check.Detail)

	textLine := lineContainingOMPDoctorCheck(t, failedText, check.ID)
	assert.Contains(t, textLine, "ERROR", "text status must mirror JSON status=fail")
	assert.Contains(t, textLine, check.ID)
	assert.Contains(t, textLine, check.Detail)
	assertOMPDoctorProjectionIsRedacted(t, root, failedText, validationChecks)

	invocations, err := os.ReadFile(logPath)
	require.NoError(t, err)
	log := string(invocations)
	for _, expected := range []string{
		"--version", "--help", "config get skills.customDirectories --json",
		"models --json", "--mode rpc", "--model autopus-readiness/readiness-probe",
		"--tools write", "loopback-provider-requests=2",
	} {
		assert.Contains(t, log, expected)
	}
	for _, forbidden := range []string{ompDoctorSecretSentinel, ompDoctorRawPayload, "Authorization"} {
		assert.NotContains(t, log, forbidden)
	}
}

func runOMPDoctorEntrypoints(t *testing.T, root string) (string, jsonEnvelope) {
	t.Helper()
	var text bytes.Buffer
	textCmd := NewRootCmd()
	textCmd.SetOut(&text)
	textCmd.SetErr(&text)
	textCmd.SetArgs([]string{"doctor", "--dir", root})
	require.NoError(t, textCmd.Execute())

	var encoded bytes.Buffer
	jsonCmd := NewRootCmd()
	jsonCmd.SetOut(&encoded)
	jsonCmd.SetErr(&encoded)
	jsonCmd.SetArgs([]string{"doctor", "--dir", root, "--json"})
	require.NoError(t, jsonCmd.Execute())
	var envelope jsonEnvelope
	require.NoError(t, json.Unmarshal(encoded.Bytes(), &envelope), encoded.String())
	return text.String(), envelope
}

func installHermeticOMPDoctorCLI(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "omp-invocations.log")
	executable, err := os.Executable()
	require.NoError(t, err)
	script := fmt.Sprintf("#!/bin/sh\nexec %q -test.run='^TestOMPDoctorBehaviorFixtureProcess$' -- --fixture-log=%q \"$@\"\n",
		executable, logPath)
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "omp"), []byte(script), 0o755))
	t.Setenv("PATH", binDir)
	return logPath
}

func TestOMPDoctorBehaviorFixtureProcess(t *testing.T) {
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		return
	}
	os.Exit(runOMPDoctorBehaviorFixture(os.Args[separator+1:]))
}

func runOMPDoctorBehaviorFixture(args []string) int {
	logPath := strings.TrimPrefix(args[0], "--fixture-log=")
	args = args[1:]
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 61
	}
	_, _ = fmt.Fprintln(log, strings.Join(args, " "))
	_ = log.Close()
	joined := strings.Join(args, " ")
	switch {
	case joined == "--version":
		fmt.Println("omp/17.1.8")
	case joined == "--help":
		fmt.Println("--mode rpc --no-session --cwd DIR --model MODEL")
	case joined == "config get skills.customDirectories --json":
		fmt.Println(`{"key":"skills.customDirectories","value":[".agents/skills"],"type":"array","description":""}`)
	case strings.Contains(joined, "models --json"):
		fmt.Println(`{"models":[{"provider":"legacy","id":"opus","available":true},{"provider":"legacy","id":"sonnet","available":true}]}`)
	case strings.Contains(joined, "--mode rpc"):
		return runOMPDoctorBehaviorRPC(args, logPath)
	default:
		return 62
	}
	return 0
}

func runOMPDoctorBehaviorRPC(args []string, logPath string) int {
	scanner := bufio.NewScanner(io.LimitReader(os.Stdin, 2049))
	var input bytes.Buffer
	for range 3 {
		if !scanner.Scan() {
			return 63
		}
		input.Write(scanner.Bytes())
		input.WriteByte('\n')
	}
	if scanner.Err() != nil || input.Len() > 2048 || !bytes.Contains(input.Bytes(), []byte(`"type":"set_auto_retry"`)) ||
		!bytes.Contains(input.Bytes(), []byte(`"type":"set_auto_compaction"`)) || !bytes.Contains(input.Bytes(), []byte(`"type":"prompt"`)) {
		return 63
	}
	modelConfig, err := os.ReadFile(filepath.Join(os.Getenv("PI_CODING_AGENT_DIR"), "models.yml"))
	if err != nil {
		return 64
	}
	baseURL := ""
	for _, line := range strings.Split(string(modelConfig), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "baseUrl:") {
			baseURL = strings.TrimSpace(strings.TrimPrefix(trimmed, "baseUrl:"))
		}
	}
	for _, stage := range []string{"first", "second readiness-receipt.json"} {
		body, _ := json.Marshal(map[string]any{"model": "readiness-probe", "stream": true,
			"tools":    []any{map[string]any{"type": "function", "function": map[string]any{"name": "write"}}},
			"messages": []any{map[string]string{"role": "user", "content": stage}}})
		response, postErr := http.Post(baseURL+"/chat/completions", "application/json", bytes.NewReader(body))
		if postErr != nil || response.StatusCode != http.StatusOK {
			return 65
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8193))
		_ = response.Body.Close()
	}
	root := args[slices.Index(args, "--cwd")+1]
	if os.WriteFile(filepath.Join(root, "readiness-receipt.json"), []byte("readiness-ok\n"), 0o600) != nil {
		return 66
	}
	log, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = fmt.Fprintln(log, "loopback-provider-requests=2")
	_ = log.Close()
	fmt.Println(`{"type":"available_commands_update"}`)
	fmt.Println(`{"type":"tool_execution_start","toolCallId":"doctor-1"}`)
	fmt.Println(`{"type":"tool_execution_end","toolCallId":"doctor-1"}`)
	fmt.Println(`{"type":"message_end"}`)
	return 0
}

func generatedOMPDoctorSelectors(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".omp", "agents"))
	require.NoError(t, err)
	seen := make(map[string]bool)
	for _, entry := range entries {
		data, readErr := os.ReadFile(filepath.Join(root, ".omp", "agents", entry.Name()))
		require.NoError(t, readErr)
		frontmatterDelimiters := 0
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "---" {
				frontmatterDelimiters++
				if frontmatterDelimiters == 2 {
					break
				}
				continue
			}
			if selector := strings.TrimSpace(strings.TrimPrefix(trimmed, "model:")); frontmatterDelimiters == 1 && strings.HasPrefix(trimmed, "model:") && selector != "" {
				seen[selector] = true
			}
		}
	}
	selectors := make([]string, 0, len(seen))
	for selector := range seen {
		selectors = append(selectors, selector)
	}
	sort.Strings(selectors)
	return selectors
}
