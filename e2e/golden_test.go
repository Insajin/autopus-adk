//go:build e2e

package e2e

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// updateGolden controls whether golden files are refreshed on this run.
// Use: go test -tags e2e -run TestGolden ./e2e/... -update
var updateGolden = flag.Bool("update", false, "update golden files")

// TestGolden validates CLI output against golden files stored in testdata/.
func TestGolden(t *testing.T) {
	t.Parallel()

	bin := buildBinary(t)

	cases := []struct {
		name   string
		args   []string
		golden string
	}{
		{
			name:   "version output",
			args:   []string{"version"},
			golden: "version_output.golden",
		},
		{
			name:   "help output",
			args:   []string{"--help"},
			golden: "help_output.golden",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := runBinary(t, bin, tc.args...)
			got := normalizeGoldenOutput(tc.golden, r.Stdout+r.Stderr)

			goldenPath := filepath.Join("testdata", tc.golden)

			if *updateGolden {
				require.NoError(t, os.MkdirAll("testdata", 0o755))
				require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o644))
				t.Logf("updated golden file: %s", goldenPath)
				return
			}

			data, err := os.ReadFile(goldenPath)
			if os.IsNotExist(err) {
				t.Skipf("golden file missing: %s (run with -update to create)", goldenPath)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, string(data), got, "output does not match golden file %s", goldenPath)
		})
	}
}

func normalizeGoldenOutput(name, got string) string {
	if name != "version_output.golden" {
		return got
	}

	versionLineRe := regexp.MustCompile(`(?m)^   .+$`)
	summaryLineRe := regexp.MustCompile(`(?m)^auto .+$`)
	pathLineRe := regexp.MustCompile(`(?m)^path: .+$`)

	got = versionLineRe.ReplaceAllString(got, "   <version>")
	got = summaryLineRe.ReplaceAllString(got, "auto <version> (commit: <commit>, built: <date>)")
	got = pathLineRe.ReplaceAllString(got, "path: <binary>")
	return got
}

// compareGolden compares got against the named golden file in testdata/.
// If the -update flag is set, it writes got to the golden file instead.
func compareGolden(t *testing.T, name, got string) {
	t.Helper()

	goldenPath := filepath.Join("testdata", name+".golden")

	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(goldenPath, []byte(got), 0o644))
		t.Logf("updated golden file: %s", goldenPath)
		return
	}

	data, err := os.ReadFile(goldenPath)
	if os.IsNotExist(err) {
		t.Skipf("golden file missing: %s (run with -update to create)", goldenPath)
		return
	}
	require.NoError(t, err)
	assert.Equal(t, string(data), got)
}
