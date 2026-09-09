package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomebrewTrustDiagnosis(t *testing.T) {
	for _, tc := range []struct {
		name, version, trust, state string
		probeErr                    bool
		cask                        bool
	}{
		{"missing", "Homebrew 6.0.22", `{"taps":[],"casks":[],"formulae":[]}`, "warn", false, true},
		{"cask trusted", "Homebrew 6.0.22", `{"taps":[],"casks":["insajin/autopus/auto"]}`, "pass", false, true},
		{"tap trusted", "Homebrew 6.0.22", `{"taps":["insajin/autopus"],"casks":[]}`, "pass", false, true},
		{"formula trust insufficient", "Homebrew 6.0.22", `{"taps":[],"casks":[],"formulae":["insajin/autopus/auto"]}`, "warn", false, true},
		{"unknown version", "Homebrew development", "", "unknown", false, true},
		{"malformed version", "not brew", "", "unknown", false, true},
		{"remote trust unresolved", "Homebrew 6.0.22", `{ "taps":["https://example.com/tap"],"casks":[]}`, "unknown", false, true},
		{"old brew", "Homebrew 5.0.0", "", "skip", false, true},
		{"no ADK tap cask", "Homebrew 6.0.22", "", "skip", false, false},
		{"failed probe", "Homebrew 6.0.22", "", "unknown", true, true},
		{"invalid json", "Homebrew 6.0.22", "oops", "unknown", false, true},
		{"missing fields", "Homebrew 6.0.22", `{}`, "unknown", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			if tc.cask {
				path := filepath.Join(repo, "Library/Taps/insajin/homebrew-autopus/Casks/auto.rb")
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
				require.NoError(t, os.WriteFile(path, []byte("raise 'must not load'"), 0644))
			}
			run := func(ctx context.Context, args ...string) ([]byte, error) {
				require.NotNil(t, ctx)
				if args[0] == "git" {
					return []byte("https://github.com/insajin/homebrew-autopus"), nil
				}
				switch strings.Join(args, " ") {
				case "--version":
					return []byte(tc.version), nil
				case "--repository":
					return []byte(repo), nil
				case "trust --json=v1":
					if tc.probeErr {
						return nil, errors.New("probe failed")
					}
					return []byte(tc.trust), nil
				default:
					t.Fatalf("unexpected or mutating brew command: %v", args)
					return nil, nil
				}
			}
			got := diagnoseHomebrewTrustWith(context.Background(), run)
			assert.Equal(t, tc.state, got.state, got.detail)
			if tc.state == "warn" {
				assert.Contains(t, got.detail, "brew trust --cask insajin/autopus/auto")
				assert.Contains(t, got.detail, "cleanup")
			}
			if tc.state == "unknown" {
				assert.NotContains(t, got.detail, "brew trust --cask")
			}
		})
	}
}

func TestHomebrewTrustNoBrew(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	assert.Equal(t, "skip", diagnoseHomebrewTrust(context.Background()).state)
}

func TestHomebrewTrustProjections(t *testing.T) {
	diagnosis := homebrewTrustDiagnosis{state: "warn", detail: "Homebrew cleanup risk; run brew trust --cask insajin/autopus/auto"}
	var out bytes.Buffer
	assert.False(t, checkHomebrewTrustText(&out, diagnosis))
	report := doctorJSONReport{status: jsonStatusOK}
	report.collectHomebrewTrustCheck(diagnosis)
	require.Len(t, report.checks, 1)
	assert.Equal(t, "doctor.homebrew.tap_trust", report.checks[0].ID)
	assert.Equal(t, "warn", report.checks[0].Status)
	assert.Equal(t, jsonStatusWarn, report.status)
	assert.Contains(t, out.String(), report.checks[0].Detail)
}

func TestHomebrewTrustProbeFailures(t *testing.T) {
	for _, failing := range []string{"--version", "--repository"} {
		t.Run(failing, func(t *testing.T) {
			d := diagnoseHomebrewTrustWith(context.Background(), func(ctx context.Context, args ...string) ([]byte, error) {
				_, bounded := ctx.Deadline()
				require.True(t, bounded, "all probes must share a bounded deadline")
				if args[0] == failing {
					return nil, context.DeadlineExceeded
				}
				return []byte("Homebrew 6.0.22"), nil
			})
			assert.Equal(t, "unknown", d.state)
			report := doctorJSONReport{status: jsonStatusOK}
			report.collectHomebrewTrustCheck(d)
			assert.Equal(t, "warn", report.checks[0].Status)
			assert.Equal(t, jsonStatusWarn, report.status)
			assert.NotContains(t, d.detail, "brew trust --cask")
		})
	}
}

func TestHomebrewTrustCustomOriginIsUnknown(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "Library/Taps/insajin/homebrew-autopus/Casks/auto.rb")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, nil, 0644))
	d := diagnoseHomebrewTrustWith(context.Background(), func(_ context.Context, args ...string) ([]byte, error) {
		switch args[0] {
		case "--version":
			return []byte("Homebrew 6.0.22"), nil
		case "--repository":
			return []byte(repo), nil
		case "git":
			return []byte("https://example.com/custom-tap"), nil
		default:
			t.Fatal("custom origin must not be certified by stale name trust")
			return nil, nil
		}
	})
	assert.Equal(t, "unknown", d.state)
}
