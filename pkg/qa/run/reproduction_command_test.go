package run

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shellRoundTrip parses a rendered line with a real shell and returns the argv
// the shell produced. Asserting against a hand-written expectation would only
// prove the renderer agrees with the test author; the property that matters is
// that a shell recovers exactly what ran.
func shellRoundTrip(t *testing.T, rendered string) []string {
	t.Helper()
	script := `printf '%s\0' ` + rendered
	out, err := exec.Command("/bin/sh", "-c", script).Output()
	require.NoError(t, err, "rendered line must parse as a shell command: %s", rendered)
	fields := strings.Split(string(out), "\x00")
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	return fields
}

func TestRenderReproductionCommand_SurvivesAShell(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"go test skip regex": {
			"go", "test", "./...", "-count=1", "-timeout=25m", "-skip",
			"^(TestReleaseHardeningBashContract|TestPOSIXInstaller)$",
		},
		"dollar and backtick": {"echo", "$HOME", "`id`", "${PATH}"},
		"quotes and spaces":   {"printf", "%s", "it's a value", `he said "hi"`},
		"redirection tokens":  {"tool", "--filter", "a>b", "x<y", "p|q", "a;b", "c&d"},
		"plain arguments":     {"go", "build", "./cmd/auto"},
		"empty argument":      {"tool", "--value", ""},
	}
	for name, argv := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, argv, shellRoundTrip(t, renderReproductionCommand(argv)))
		})
	}
}

// A naive join is the regression this renderer exists to prevent: the pasted
// line has to run one command, not a pipeline the harness never executed.
func TestRenderReproductionCommand_RegexIsNotAPipeline(t *testing.T) {
	t.Parallel()
	argv := []string{"go", "test", "./...", "-skip", "^(TestA|TestB)$"}
	rendered := renderReproductionCommand(argv)

	// The contrast is the point. Asserting on the rendered text would only
	// restate the implementation, so compare behaviour: the naive join the
	// harness used to store does not survive a shell, and this does.
	require.NotEqual(t, strings.Join(argv, " "), rendered)
	assert.Equal(t, argv, shellRoundTrip(t, rendered))

	naive := exec.Command("/bin/sh", "-c", `printf '%s\0' `+strings.Join(argv, " "))
	if out, err := naive.Output(); err == nil {
		recovered := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
		assert.NotEqual(t, argv, recovered,
			"if the naive join round-trips there is nothing for this renderer to fix")
	}
}

// Readability is a real requirement: an operator reads this field far more
// often than they paste it, so ordinary arguments must not acquire quotes.
func TestRenderReproductionCommand_LeavesOrdinaryArgumentsBare(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "go test ./... -count=1 -timeout=25m",
		renderReproductionCommand([]string{"go", "test", "./...", "-count=1", "-timeout=25m"}))
}
