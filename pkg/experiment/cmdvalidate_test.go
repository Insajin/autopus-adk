package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCommand_ShellMetacharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cmd     string
		wantErr bool
	}{
		{"safe command", "echo hello", false},
		{"semicolon injection", "echo ok; rm -rf /", true},
		{"pipe injection", "cat file | grep x", true},
		{"subshell injection", "echo $(whoami)", true},
		{"backtick injection", "echo `id`", true},
		{"double ampersand", "echo ok && echo bad", true},
		{"background operator", "sleep 999 & echo ok", true},
		{"double pipe", "echo ok || echo bad", true},
		{"brace expansion", "echo {a,b}", true},
		{"input redirect", "cat < /etc/passwd", true},
		{"output redirect", "echo > /tmp/pwned", true},
		{"newline injection", "echo ok\nrm -rf /", true},
		{"quoted JSON backward compat", `echo '{"metric": 1.5}'`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCommand(tt.cmd)

			if tt.wantErr {
				require.Error(t, err, "expected error for command: %s", tt.cmd)
				assert.Contains(t, err.Error(), "disallowed",
					"error message should mention 'disallowed' for: %s", tt.cmd)
			} else {
				assert.NoError(t, err, "expected no error for command: %s", tt.cmd)
			}
		})
	}
}

func TestValidateCommand_AllowShellMeta_Bypass(t *testing.T) {
	t.Parallel()

	// When AllowShellMeta option is set, metacharacters should be allowed.
	cmd := "echo ok; rm -rf /"
	err := ValidateCommand(cmd, AllowShellMeta())

	assert.NoError(t, err,
		"AllowShellMeta should bypass validation for: %s", cmd)
}

func TestValidateCommand_EmptyCommand(t *testing.T) {
	t.Parallel()

	err := ValidateCommand("")
	// Empty command should not contain shell metacharacters, so no error.
	assert.NoError(t, err, "empty command should be valid")
}

func TestValidateCommand_ComplexSafeCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
	}{
		{"simple binary", "go test ./..."},
		{"flags with values", "benchmark --count=5 --timeout=30s"},
		{"path arguments", "/usr/local/bin/metric-tool -o result.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCommand(tt.cmd)
			assert.NoError(t, err, "safe command should pass: %s", tt.cmd)
		})
	}
}

func TestValidateCommand_ParenthesisInjection(t *testing.T) {
	t.Parallel()

	// Parentheses are disallowed as single-char metacharacters.
	err := ValidateCommand("echo (test)")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disallowed")
}

func TestValidateCommand_SingleQuotedBracesAllowed(t *testing.T) {
	t.Parallel()

	// Content inside single quotes should be stripped before validation,
	// so braces in JSON are safe.
	err := ValidateCommand("echo '{\"key\": \"value\"}'")
	assert.NoError(t, err, "single-quoted JSON braces should be allowed")
}

func TestValidateCommand_UnmatchedSingleQuote(t *testing.T) {
	t.Parallel()

	// Unmatched single quote: original string is used for validation
	// to prevent metacharacters from being hidden by an open quote.
	err := ValidateCommand("echo 'partial | still quoted")
	require.Error(t, err, "unmatched quote must not hide metacharacters")
	assert.Contains(t, err.Error(), "disallowed")
}

func TestValidateCommand_AllowShellMeta_AllMetachars(t *testing.T) {
	t.Parallel()

	// AllowShellMeta should bypass ALL metacharacter checks.
	dangerousCmd := "rm -rf / ; echo $(whoami) | cat `id` && echo {a} || true > /dev/null < /etc/passwd"
	err := ValidateCommand(dangerousCmd, AllowShellMeta())
	assert.NoError(t, err, "AllowShellMeta should allow all metacharacters")
}

func TestValidateCommand_ErrorMessageIncludesSpecificChar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmd      string
		wantChar string
	}{
		{"semicolon identified", "echo; bad", ";"},
		{"pipe identified", "cat | grep", "|"},
		{"background operator identified", "sleep 1 & echo ok", "&"},
		{"double-ampersand identified", "a && b", "&&"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCommand(tt.cmd)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantChar,
				"error should identify the specific metacharacter")
		})
	}
}
