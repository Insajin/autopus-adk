package journey

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateCommandAcceptsAdapterArgvAllowlist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		adapterID string
		argv      []string
	}{
		{name: "direct vitest", adapterID: "vitest", argv: []string{"vitest", "--run"}},
		{name: "npx vitest", adapterID: "vitest", argv: []string{"npx", "vitest", "--run"}},
		{name: "npm exec vitest", adapterID: "vitest", argv: []string{"npm", "exec", "vitest", "--", "--run"}},
		{name: "yarn jest", adapterID: "jest", argv: []string{"yarn", "jest", "--runInBand"}},
		{name: "playwright test", adapterID: "playwright", argv: []string{"npx", "playwright", "test"}},
		{name: "auto test run", adapterID: "auto-test-run", argv: []string{"auto", "test", "run", "--lane", "fast"}},
		{name: "auto verify", adapterID: "auto-verify", argv: []string{"auto", "verify", "--target", "local"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCommand(tt.adapterID, Command{Argv: tt.argv, CWD: ".", Timeout: "60s"}, nil, t.TempDir(), "qa_journey")

			require.NoError(t, err)
		})
	}
}

func TestValidateCommandRejectsAdapterArgvBypasses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		adapterID string
		argv      []string
	}{
		{name: "vitest through jest", adapterID: "vitest", argv: []string{"npx", "jest"}},
		{name: "jest through vitest", adapterID: "jest", argv: []string{"npm", "exec", "vitest"}},
		{name: "playwright missing test", adapterID: "playwright", argv: []string{"npx", "playwright"}},
		{name: "node install", adapterID: "node-script", argv: []string{"npm", "install"}},
		{name: "node binary", adapterID: "node-script", argv: []string{"node", "test.js"}},
		{name: "auto test incomplete", adapterID: "auto-test-run", argv: []string{"auto", "test"}},
		{name: "auto verify mismatch", adapterID: "auto-verify", argv: []string{"auto", "test", "run"}},
		{name: "unknown adapter", adapterID: "unknown", argv: []string{"go", "test", "./..."}},
		{name: "path qualified go", adapterID: "go-test", argv: []string{"./tools/go", "test", "./..."}},
		{name: "path qualified npx", adapterID: "vitest", argv: []string{"/usr/bin/npx", "vitest"}},
		{name: "path qualified auto", adapterID: "auto-verify", argv: []string{"./bin/auto", "verify"}},
		{name: "custom env split shell", adapterID: "custom-command", argv: []string{"env", "-S", "sh -c id"}},
		{name: "custom run without argv", adapterID: "custom-command", argv: nil},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			command := Command{Argv: tt.argv, CWD: ".", Timeout: "60s"}
			if tt.name == "custom run without argv" {
				command.Run = "go test ./..."
			}
			err := ValidateCommand(tt.adapterID, command, nil, t.TempDir(), "qa_journey")

			require.Error(t, err)
		})
	}
}
