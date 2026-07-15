package cli

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestClassifyOperationalError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		stderr string
		err    error
		want   string
	}{
		{"binary", "", errors.New("executable file not found"), "binary_missing"},
		{"config", "error: unexpected argument --bad", errors.New("exit status 2"), "cli_usage_or_config"},
		{"auth", "401 Unauthorized: login required", errors.New("exit status 1"), "authentication"},
		{"model", "model gpt-x is not available for this account", errors.New("exit status 1"), "model_access"},
		{"network", "TLS handshake timeout", errors.New("exit status 1"), "network_transport"},
		{"provider", "429 rate limit exceeded", errors.New("exit status 1"), "provider_rejected"},
		{"schema", "output schema response parse failed", errors.New("exit status 1"), "schema_or_response"},
		{"unknown", "opaque failure", errors.New("exit status 9"), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class, fingerprint := classifyOperationalError(tc.stderr, tc.err)
			if class != tc.want {
				t.Fatalf("class=%q want=%q", class, tc.want)
			}
			if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(fingerprint) {
				t.Fatalf("unsafe fingerprint shape: %q", fingerprint)
			}
			for _, secret := range []string{"Unauthorized", "gpt-x", "opaque failure"} {
				if strings.Contains(fingerprint, secret) {
					t.Fatalf("fingerprint retained raw input: %q", fingerprint)
				}
			}
		})
	}
}

func TestBoundedOperationalErrorBuffer(t *testing.T) {
	t.Parallel()
	var buffer boundedOperationalErrorBuffer
	payload := strings.Repeat("secret", 20000)
	n, err := buffer.Write([]byte(payload))
	if err != nil || n != len(payload) {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if len(buffer.String()) != maxOperationalErrorBytes {
		t.Fatalf("buffer size=%d want=%d", len(buffer.String()), maxOperationalErrorBytes)
	}
	if !buffer.HasData() {
		t.Fatal("buffer did not report bounded data")
	}
}

func TestOperationalErrorSignalsAreCanonicalAndRawFree(t *testing.T) {
	t.Parallel()
	cases := []struct {
		stderr, provider, parse bool
		want                    []string
	}{
		{want: nil},
		{stderr: true, want: []string{"stderr"}},
		{provider: true, want: []string{"provider_failure_event"}},
		{parse: true, want: []string{"stream_parse_failure"}},
		{stderr: true, provider: true, parse: true,
			want: []string{"stderr", "provider_failure_event", "stream_parse_failure"}},
	}
	for _, tc := range cases {
		got := operationalErrorSignals(tc.stderr, tc.provider, tc.parse)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("signals=%v want=%v", got, tc.want)
		}
	}
}

func TestProviderFailureObservationIsStructuralAndRawFree(t *testing.T) {
	t.Parallel()
	var observed providerFailureObservation
	observed.Observe("error", "", []byte(`{"type":"error","message":"RAW-SECRET"}`))
	observed.Observe("turn", "failed", []byte(`{"type":"turn.failed","status":500,"error":{"message":"NESTED-SECRET","type":"opaque","code":"private"}}`))
	if observed.Kind() != "error_and_turn_failed" {
		t.Fatalf("kind=%q", observed.Kind())
	}
	want := []string{"top_level_message", "top_level_status", "nested_error_object", "nested_error_message", "nested_error_type", "nested_error_code"}
	if strings.Join(observed.Shape(), ",") != strings.Join(want, ",") {
		t.Fatalf("shape=%v want=%v", observed.Shape(), want)
	}
	serialized := observed.Kind() + strings.Join(observed.Shape(), ",")
	for _, secret := range []string{"RAW-SECRET", "NESTED-SECRET", "opaque", "private", "500"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("structural receipt retained value %q", secret)
		}
	}
}

func TestOperationalErrorNumericBoundariesAndMissingFileContext(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		message string
		want    string
	}{
		{"request failed (401)", "authentication"},
		{"build 14019 failed", "unknown"},
		{"counter 14290 exceeded", "unknown"},
		{"config schema: no such file or directory", "unknown"},
		{"fork/exec /tmp/codex: no such file or directory", "binary_missing"},
	} {
		got, _ := classifyOperationalError(tc.message, nil)
		if got != tc.want {
			t.Fatalf("message=%q class=%q want=%q", tc.message, got, tc.want)
		}
	}
}

func TestBuildAgentTaskConfigCarriesExplicitEvidenceMode(t *testing.T) {
	t.Parallel()
	ctx := taskContext{EvidenceMode: true}
	if !buildAgentTaskConfig("task", "run", ctx).EvidenceMode {
		t.Fatal("evidence mode was not propagated to execution config")
	}
	ctx.EvidenceMode = false
	if buildAgentTaskConfig("task", "run", ctx).EvidenceMode {
		t.Fatal("legacy execution was promoted to evidence mode")
	}
}
