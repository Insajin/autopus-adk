package journey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pack has to be able to express a test selector. Go's own -run and -skip
// flags take regexes, so refusing alternation and anchors in argv refused the
// ordinary way to split a suite. argv is handed to exec with no shell, so these
// bytes never reach an interpreter.
func TestValidateArgv_AcceptsRegexArguments(t *testing.T) {
	t.Parallel()
	for name, argv := range map[string][]string{
		"skip alternation": {
			"go", "test", "./...", "-skip",
			"^(TestReleaseHardeningBashContract|TestPOSIXInstaller)$",
		},
		"run anchor":      {"go", "test", "./...", "-run", "^TestOnlyThis$"},
		"jsonpath filter": {"tool", "--query", "$.items[0].name"},
		"glob argument":   {"tool", "--paths", "pkg/**|internal/**"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, validateArgv(argv))
		})
	}
}

// Control characters stay refused. argv is written into logs and manifest
// fields, where a newline invents a line that nothing produced and a NUL
// truncates the value.
func TestValidateArgv_RefusesControlCharacters(t *testing.T) {
	t.Parallel()
	for name, argv := range map[string][]string{
		"newline":         {"go", "test", "./...\nrm -rf /"},
		"carriage return": {"go", "test", "a\rb"},
		"nul":             {"go", "test", "a\x00b"},
		"escape":          {"go", "test", "a\x1bb"},
		"delete":          {"go", "test", "a\x7fb"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateArgv(argv)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "control characters")
		})
	}
}

// The shell-invocation refusal is what makes the relaxation safe: argv may
// carry shell syntax precisely because no shell runs it.
func TestValidateArgv_StillRefusesShellInvocation(t *testing.T) {
	t.Parallel()
	for name, argv := range map[string][]string{
		"sh -c":   {"sh", "-c", "go test ./..."},
		"bash -c": {"bash", "-c", "echo hi"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateArgv(argv)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "may not invoke a shell")
		})
	}
}

// command.run is a shell string, so its ban is not relaxed. Losing this
// distinction is how the argv change would turn into an injection path.
func TestValidateCommandShape_RunKeepsTheShellBan(t *testing.T) {
	t.Parallel()
	err := validateCommandShape("custom-command", Command{Run: "go test ./... | tee log"})
	require.Error(t, err)
}
