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
	// root, where it keeps the per-account Codex homes it swaps CODEX_HOME to.
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
	// maxCredentialBytes bounds a credential read. An auth.json is a few
	// kilobytes; a larger file is a malfunction and is refused rather than read.
	maxCredentialBytes = 1 << 20
)

// Credential scopes name a credential location for humans. They are used in
// place of the path, which embeds the managed account identifier.
const (
	executionCredentialScope = "orca-managed codex home"
	probeCredentialScope     = "local codex home"
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

// managedCodexAuthPath builds the credential path of one orca-managed account
// under supportRoot, the user's application support directory. The root is a
// parameter so a test can point the layout at a temporary tree.
func managedCodexAuthPath(supportRoot, accountID string) (string, error) {
	if !managedAccountIDPattern.MatchString(accountID) {
		return "", ErrUnsafeAccountID
	}
	if supportRoot == "" {
		return "", fmt.Errorf("%w: no application support directory to locate the %s",
			ErrCredentialUnavailable, executionCredentialScope)
	}
	return filepath.Join(supportRoot, orcaSupportDir, orcaCodexAccountsDir,
		accountID, orcaManagedHomeDir, codexAuthFileName), nil
}

// localCodexAuthPath builds the credential path of the PATH codex CLI. An
// explicit codexHome wins, matching how the CLI itself resolves its home.
func localCodexAuthPath(codexHome, userHome string) (string, error) {
	if codexHome != "" {
		return filepath.Join(codexHome, codexAuthFileName), nil
	}
	if userHome == "" {
		return "", fmt.Errorf("%w: no home directory to locate the %s",
			ErrCredentialUnavailable, probeCredentialScope)
	}
	return filepath.Join(userHome, localCodexHomeDir, codexAuthFileName), nil
}

// readManagedCodexAuth reads the credential of the account that will run the
// workload. It is the default Prober.Credential seam.
func readManagedCodexAuth(accountID string) ([]byte, error) {
	supportRoot, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, executionCredentialScope, err)
	}
	path, err := managedCodexAuthPath(supportRoot, accountID)
	if err != nil {
		return nil, err
	}
	return readCredentialFile(path, executionCredentialScope)
}

// readLocalCodexAuth reads the credential the held catalog was probed under. It
// is the default Prober.HostCredential seam.
func readLocalCodexAuth() ([]byte, error) {
	codexHome := os.Getenv(codexHomeEnv)
	userHome, homeErr := os.UserHomeDir()
	if codexHome == "" && homeErr != nil {
		return nil, fmt.Errorf("%w: cannot locate the %s: %w",
			ErrCredentialUnavailable, probeCredentialScope, homeErr)
	}
	path, err := localCodexAuthPath(codexHome, userHome)
	if err != nil {
		return nil, err
	}
	return readCredentialFile(path, probeCredentialScope)
}

// readCredentialFile reads a bounded credential. A missing or unreadable file
// is an ordinary outcome here: it returns an identifiable reason so the caller
// degrades to an unknown grade instead of failing the run.
func readCredentialFile(path, scope string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, credentialReadError(scope, err)
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, maxCredentialBytes+1))
	if err != nil {
		return nil, credentialReadError(scope, err)
	}
	if len(payload) > maxCredentialBytes {
		return nil, fmt.Errorf("%w: credential in the %s exceeds %d bytes",
			ErrCredentialUnavailable, scope, maxCredentialBytes)
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
