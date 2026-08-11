package execplane

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

const (
	// orcaSupportDir is orca's directory under the user's application support
	// root, where it keeps the per-account provider state it swaps in.
	orcaSupportDir = "orca"
	// orcaCodexAccountsDir holds one managed home per Codex account id.
	orcaCodexAccountsDir = "codex-accounts"
	// orcaManagedHomeDir is the CODEX_HOME orca points a worker at.
	orcaManagedHomeDir = "home"
	// localCodexHomeDir is the default CODEX_HOME of the PATH codex CLI.
	localCodexHomeDir = ".codex"
	// codexAuthFileName holds the credential whose id_token carries the grade.
	codexAuthFileName = "auth.json"
	// codexHomeEnv overrides the local codex home.
	codexHomeEnv = "CODEX_HOME"
)

const (
	// orcaClaudeAccountsDir holds one managed credential per Claude account id.
	orcaClaudeAccountsDir = "claude-accounts"
	// orcaClaudeAuthDir is the credential directory inside a managed account.
	orcaClaudeAuthDir = "auth"
	// claudeAuthFileName holds the signed in account's plan fields. Claude
	// records the grade in the account record rather than inside a token.
	claudeAuthFileName = "oauth-account.json"
	// localClaudeConfigFile is the configuration of the PATH claude CLI. It
	// nests the same account fields under `oauthAccount`.
	localClaudeConfigFile = ".claude.json"
	// claudeConfigDirEnv relocates the local claude configuration directory,
	// the way CODEX_HOME relocates the codex home.
	claudeConfigDirEnv = "CLAUDE_CONFIG_DIR"
)

const (
	// maxCredentialBytes bounds a credential read. A credential file is a few
	// kilobytes; a larger one is a malfunction and is refused rather than read.
	maxCredentialBytes = 1 << 20
	// maxHostConfigBytes bounds the local claude configuration, which is not a
	// bare credential: it accumulates per-project state beside the account
	// fields, so a real one outgrows a credential by orders of magnitude.
	maxHostConfigBytes = 16 << 20
)

// Credential scopes name a credential location for humans. They are used in
// place of the path, which embeds the managed account identifier.
const (
	codexExecutionScope  = "orca-managed codex home"
	codexProbeScope      = "local codex home"
	claudeExecutionScope = "orca-managed claude account"
	claudeProbeScope     = "local claude configuration"
)

var (
	// ErrCredentialUnavailable reports that a credential could not be read. The
	// grade stays unknown, which the gate reports as unverified — it never
	// stands in for a permissive default.
	ErrCredentialUnavailable = errors.New("execplane: provider credential is unavailable")
	// ErrUnsafeAccountID reports an account identifier that is not a UUID. It is
	// refused before it is joined into a path, because the identifier comes from
	// a subprocess payload and a traversal in it would escape the managed root.
	ErrUnsafeAccountID = errors.New("execplane: managed account identifier is not a uuid")
)

// managedAccountIDPattern accepts exactly the shape orca issues. Anything else,
// including a path separator or a dot segment, is refused.
var managedAccountIDPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// managedAccountPath joins a location inside one orca-managed account under
// supportRoot, the user's application support directory. The root is a
// parameter so a test can point the layout at a temporary tree, and every
// provider reaches its managed state through this one validation.
func managedAccountPath(supportRoot, accountsDir, accountID, scope string, tail ...string) (string, error) {
	if !managedAccountIDPattern.MatchString(accountID) {
		return "", ErrUnsafeAccountID
	}
	if supportRoot == "" {
		return "", fmt.Errorf("%w: no application support directory to locate the %s",
			ErrCredentialUnavailable, scope)
	}
	elements := make([]string, 0, 4+len(tail))
	elements = append(elements, supportRoot, orcaSupportDir, accountsDir, accountID)
	return filepath.Join(append(elements, tail...)...), nil
}

// managedCodexHomePath builds the CODEX_HOME orca swaps in for one account.
func managedCodexHomePath(supportRoot, accountID string) (string, error) {
	return managedAccountPath(supportRoot, orcaCodexAccountsDir, accountID,
		codexExecutionScope, orcaManagedHomeDir)
}

// ManagedCodexHome resolves the CODEX_HOME of one orca-managed account against
// the real application support directory. A re-probe runs the codex CLI under
// this home so the catalog it returns is the execution account's own. Only the
// path is assembled here; nothing under it is read or written.
func ManagedCodexHome(accountID string) (string, error) {
	supportRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, codexExecutionScope, err)
	}
	return managedCodexHomePath(supportRoot, accountID)
}

// managedCodexAuthPath builds the credential path of one orca-managed Codex
// account, which lives inside that account's managed home.
func managedCodexAuthPath(supportRoot, accountID string) (string, error) {
	return managedAccountPath(supportRoot, orcaCodexAccountsDir, accountID,
		codexExecutionScope, orcaManagedHomeDir, codexAuthFileName)
}

// managedClaudeAuthPath builds the credential path of one orca-managed Claude
// account. Claude keeps the account record in its own credential directory
// rather than in a CLI home, so the layout differs below the account id.
func managedClaudeAuthPath(supportRoot, accountID string) (string, error) {
	return managedAccountPath(supportRoot, orcaClaudeAccountsDir, accountID,
		claudeExecutionScope, orcaClaudeAuthDir, claudeAuthFileName)
}

// localCodexAuthPath builds the credential path of the PATH codex CLI. An
// explicit codexHome wins, matching how the CLI itself resolves its home.
func localCodexAuthPath(codexHome, userHome string) (string, error) {
	if codexHome != "" {
		return filepath.Join(codexHome, codexAuthFileName), nil
	}
	if userHome == "" {
		return "", fmt.Errorf("%w: no home directory to locate the %s",
			ErrCredentialUnavailable, codexProbeScope)
	}
	return filepath.Join(userHome, localCodexHomeDir, codexAuthFileName), nil
}

// localClaudeConfigPath builds the configuration path of the PATH claude CLI.
// CLAUDE_CONFIG_DIR relocates that CLI's configuration directory, so it wins
// when set.
func localClaudeConfigPath(configDir, userHome string) (string, error) {
	if configDir != "" {
		return filepath.Join(configDir, localClaudeConfigFile), nil
	}
	if userHome == "" {
		return "", fmt.Errorf("%w: no home directory to locate the %s",
			ErrCredentialUnavailable, claudeProbeScope)
	}
	return filepath.Join(userHome, localClaudeConfigFile), nil
}

// readManagedCodexAuth reads the credential of the Codex account that will run
// the workload.
func readManagedCodexAuth(accountID string) ([]byte, error) {
	supportRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, codexExecutionScope, err)
	}
	path, err := managedCodexAuthPath(supportRoot, accountID)
	if err != nil {
		return nil, err
	}
	return readCredentialFile(path, codexExecutionScope, maxCredentialBytes)
}

// readManagedClaudeAuth reads the credential of the Claude account that will
// run the workload.
func readManagedClaudeAuth(accountID string) ([]byte, error) {
	supportRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, claudeExecutionScope, err)
	}
	path, err := managedClaudeAuthPath(supportRoot, accountID)
	if err != nil {
		return nil, err
	}
	return readCredentialFile(path, claudeExecutionScope, maxCredentialBytes)
}

// readLocalCodexAuth reads the credential the held Codex catalog was probed
// under, which is the PATH CLI's login rather than the execution account.
func readLocalCodexAuth() ([]byte, error) {
	codexHome := os.Getenv(codexHomeEnv)
	userHome, homeErr := os.UserHomeDir()
	if codexHome == "" && homeErr != nil {
		return nil, fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, codexProbeScope, homeErr)
	}
	path, err := localCodexAuthPath(codexHome, userHome)
	if err != nil {
		return nil, err
	}
	return readCredentialFile(path, codexProbeScope, maxCredentialBytes)
}

// readLocalClaudeConfig reads the configuration the held Claude catalog was
// probed under. A relocated configuration directory does not always carry the
// account record — which of the two locations holds it depends on the CLI
// version — so the home copy is retried as the same source, not a second one.
func readLocalClaudeConfig() ([]byte, error) {
	configDir := os.Getenv(claudeConfigDirEnv)
	userHome, homeErr := os.UserHomeDir()
	if configDir == "" && homeErr != nil {
		return nil, fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, claudeProbeScope, homeErr)
	}
	path, err := localClaudeConfigPath(configDir, userHome)
	if err != nil {
		return nil, err
	}
	payload, err := readCredentialFile(path, claudeProbeScope, maxHostConfigBytes)
	if err == nil || configDir == "" || userHome == "" {
		return payload, err
	}
	homePath, homePathErr := localClaudeConfigPath("", userHome)
	if homePathErr != nil {
		return nil, err
	}
	return readCredentialFile(homePath, claudeProbeScope, maxHostConfigBytes)
}

// readCredentialFile reads a bounded credential. A missing or unreadable file
// is an ordinary outcome here: it returns an identifiable reason so the caller
// degrades to an unknown grade instead of failing the run.
func readCredentialFile(path, scope string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, credentialReadError(scope, err)
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, credentialReadError(scope, err)
	}
	if int64(len(payload)) > maxBytes {
		return nil, fmt.Errorf("%w: credential in the %s exceeds %d bytes",
			ErrCredentialUnavailable, scope, maxBytes)
	}
	return payload, nil
}

// credentialReadError names the location but never the path, because a managed
// path carries the account identifier and this error reaches logs.
func credentialReadError(scope string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: no credential file in the %s", ErrCredentialUnavailable, scope)
	}
	return fmt.Errorf("%w: credential file in the %s could not be read",
		ErrCredentialUnavailable, scope)
}
