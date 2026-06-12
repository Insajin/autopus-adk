package domainreadiness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// validatesCommand runs validateCommandShape via a scenario-shaped command and
// reports whether "invented_command" was raised.
func inventedCommand(adapter string, argv []string) bool {
	findings := validateCommandShape(CommandShape{Adapter: adapter, Argv: argv})
	for _, f := range findings {
		if f == "invented_command" {
			return true
		}
	}
	return false
}

// TestKnownAdapterCommandRecognizesEachAdapter asserts the known-good argv for each
// supported adapter is NOT flagged as invented, covering all switch arms.
func TestKnownAdapterCommandRecognizesEachAdapter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		adapter string
		argv    []string
	}{
		{"go-test", "go-test", []string{"go", "test", "./..."}},
		{"node-script test", "node-script", []string{"npm", "test"}},
		{"node-script run", "node-script", []string{"pnpm", "run", "ci"}},
		{"playwright direct", "playwright", []string{"playwright", "test"}},
		{"playwright npx", "playwright", []string{"npx", "playwright", "test"}},
		{"gui-explore npm exec", "gui-explore", []string{"npm", "exec", "playwright"}},
		{"vitest direct", "vitest", []string{"vitest", "run"}},
		{"vitest yarn", "vitest", []string{"yarn", "vitest"}},
		{"jest", "jest", []string{"jest"}},
		{"pytest direct", "pytest", []string{"pytest", "-q"}},
		{"pytest python -m", "pytest", []string{"python", "-m", "pytest"}},
		{"cargo-test", "cargo-test", []string{"cargo", "test"}},
		{"auto-test-run", "auto-test-run", []string{"auto", "test", "run"}},
		{"auto-verify", "auto-verify", []string{"auto", "verify"}},
		{"canary-template", "canary-template", []string{"auto", "canary", "run"}},
		{"custom-command", "custom-command", []string{"maestro", "test"}},
		{"empty argv allowed", "go-test", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.False(t, inventedCommand(tc.adapter, tc.argv), "expected %v to be recognized", tc.argv)
		})
	}
}

// TestKnownAdapterCommandRejectsMismatched asserts unknown adapters and mismatched
// argv are flagged as invented commands.
func TestKnownAdapterCommandRejectsMismatched(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		adapter string
		argv    []string
	}{
		{"unknown adapter", "mystery", []string{"foo"}},
		{"go-test wrong subcmd", "go-test", []string{"go", "build"}},
		{"node-script wrong pm", "node-script", []string{"deno", "test"}},
		{"pytest wrong", "pytest", []string{"ruby", "spec"}},
		{"cargo wrong", "cargo-test", []string{"cargo", "build"}},
		{"custom unknown exe", "custom-command", []string{"danger"}},
		{"playwright wrong runner", "vitest", []string{"npx", "jest"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, inventedCommand(tc.adapter, tc.argv), "expected %v to be invented", tc.argv)
		})
	}
}

// TestKnownExecutableAllowlist asserts the custom-command executable allowlist.
func TestKnownExecutableAllowlist(t *testing.T) {
	t.Parallel()

	for _, ok := range []string{"go", "npm", "pnpm", "yarn", "npx", "pytest", "python", "cargo", "auto", "maestro", "appium"} {
		assert.True(t, knownExecutable(ok), ok)
	}
	for _, bad := range []string{"rm", "curl", "bash", ""} {
		assert.False(t, knownExecutable(bad), bad)
	}
}

// TestUnsafeEnvAllowlist asserts secret-like and malformed env names are unsafe.
func TestUnsafeEnvAllowlist(t *testing.T) {
	t.Parallel()

	for _, unsafe := range []string{"", "FOO=bar", "HAS SPACE", "API_KEY", "MY_SECRET", "PASSWORD_X", "$VAR"} {
		assert.True(t, unsafeEnvAllowlist(unsafe), unsafe)
	}
	for _, safe := range []string{"HOME", "PATH", "CI"} {
		assert.False(t, unsafeEnvAllowlist(safe), safe)
	}
}
