package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// --- Helper / util tests ---

func TestNormalizeModelID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"opus-4.7", "opus-4-7"},
		{"claude-opus-4.7", "opus-4-7"},
		{"haiku-4.5", "haiku-4-5"},
		{"haiku-4-5", "haiku-4-5"},
		{"sonnet-4.6", "sonnet-4-6"},
		{"OPUS-4.7", "opus-4-7"},
	}
	for _, c := range cases {
		got := normalizeModelID(c.in)
		if got != c.want {
			t.Errorf("normalizeModelID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveModel_EnvFallback(t *testing.T) {
	t.Setenv(modelEnvKey, "opus-4.7")
	got := resolveModel("")
	if got != "opus-4.7" {
		t.Errorf("resolveModel() with env = %q, want opus-4.7", got)
	}
}

func TestResolveModel_DefaultFallback(t *testing.T) {
	_ = os.Unsetenv(modelEnvKey)
	got := resolveModel("")
	if got != defaultModel {
		t.Errorf("resolveModel() = %q, want %q", got, defaultModel)
	}
}

func TestEffortResolve_SettingsDefault(t *testing.T) {
	// No env, no flag, no quality → settings_default medium.
	result, err := ResolveEffort(EffortResolveInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Effort != EffortMedium {
		t.Errorf("effort: got %q, want medium", result.Effort)
	}
	if result.Source != EffortSourceSettingsDefault {
		t.Errorf("source: got %q, want settings_default", result.Source)
	}
}

func TestEffortResolve_FlagBypassesEnumGuard(t *testing.T) {
	result, err := ResolveEffort(EffortResolveInput{FlagEffort: "extreme"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Effort != EffortValue("extreme") {
		t.Errorf("effort: got %q, want %q", result.Effort, "extreme")
	}
	if result.Source != EffortSourceFlag {
		t.Errorf("source: got %q, want %q", result.Source, EffortSourceFlag)
	}
}

func TestEffortResolve_UnknownQuality(t *testing.T) {
	_, err := ResolveEffort(EffortResolveInput{FlagQuality: "turbo"})
	if err == nil {
		t.Fatal("expected error for unknown quality mode")
	}
}

func TestIsValidEffort(t *testing.T) {
	valid := []EffortValue{EffortLow, EffortMedium, EffortHigh, EffortXHigh}
	for _, v := range valid {
		if !isValidEffort(v) {
			t.Errorf("isValidEffort(%q) should be true", v)
		}
	}
	if isValidEffort("max") {
		t.Error("isValidEffort(max) should be false")
	}
	if isValidEffort("") {
		t.Error("isValidEffort('') should be false")
	}
}

// --- CLI command integration tests ---

func TestEffortDetectCmd_PlainOutput(t *testing.T) {
	cmd := newEffortDetectCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--quality=ultra", "--model=opus-4.7", "--format=plain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "effort=xhigh" {
		t.Errorf("output: got %q, want %q", out, "effort=xhigh")
	}
}

func TestEffortDetectCmd_JSONOutput(t *testing.T) {
	cmd := newEffortDetectCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--quality=ultra", "--model=opus-4.7", "--format=json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if m["effort"] != "xhigh" {
		t.Errorf("json effort: got %v", m["effort"])
	}
}

func TestEffortDetectCmd_Stripped_PlainOutput(t *testing.T) {
	cmd := newEffortDetectCmd()
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--quality=ultra", "--model=haiku-4.5", "--format=plain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "effort=" {
		t.Errorf("output: got %q, want %q", out, "effort=")
	}
	if !strings.Contains(errBuf.String(), "haiku-4-5") {
		t.Errorf("stderr should mention haiku-4-5, got: %q", errBuf.String())
	}
}

func TestEffortDetectCmd_Balanced_Complexity_High(t *testing.T) {
	cmd := newEffortDetectCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--quality=balanced", "--complexity=high", "--format=plain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "effort=high" {
		t.Errorf("output: got %q, want effort=high", out)
	}
}

func TestEffortDetectCmd_EnvOverride(t *testing.T) {
	t.Setenv(envEffortKey, "low")
	defer func() { _ = os.Unsetenv(envEffortKey) }()

	cmd := newEffortDetectCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--quality=ultra", "--model=opus-4.7", "--format=plain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != "effort=low" {
		t.Errorf("output: got %q, want effort=low", out)
	}
}

func TestEffortDetectCmd_InvalidEnvWarnsAndFallsBack(t *testing.T) {
	t.Setenv(envEffortKey, "max")
	defer func() { _ = os.Unsetenv(envEffortKey) }()

	cmd := newEffortDetectCmd()
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--quality=balanced", "--model=opus-4.7", "--format=plain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if out := strings.TrimSpace(buf.String()); out != "effort=medium" {
		t.Errorf("output: got %q, want effort=medium", out)
	}
	if !strings.Contains(errBuf.String(), "outside SPEC-CC21-001 enum") {
		t.Errorf("stderr should contain fallback warning, got: %q", errBuf.String())
	}
}
