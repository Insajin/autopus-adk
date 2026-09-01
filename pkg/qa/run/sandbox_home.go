package run

import (
	"os"
	"path/filepath"
	"strings"
)

// sandboxHomeDirName is the HOME a pack command sees when its Journey Pack does
// not allowlist the real HOME.
//
// It lives under the harness-owned cache root rather than at the project root.
// Build tools treat HOME as writable state (Go writes
// `Library/Application Support/go/telemetry`, npm writes `.npm`, pip writes
// `.cache`), so pointing HOME at the project directory dumped tool state into
// the user's repository. Keeping the hermetic intent while containing the writes
// means redirecting HOME to a directory the harness already owns and gitignores.
const sandboxHomeDirName = "sandbox-home"

// ensureSandboxHome returns an existing, harness-owned HOME for pack commands.
// The directory is created eagerly because some tools refuse to run when HOME
// does not exist, and it is validated for symlink components so a pre-planted
// link cannot redirect tool state outside the cache root.
func ensureSandboxHome(paths goCachePaths) string {
	home := filepath.Join(paths.Root, sandboxHomeDirName)
	if err := os.MkdirAll(home, 0o755); err == nil && sandboxHomeContained(paths, home) {
		return home
	}
	fallback := filepath.Join(os.TempDir(), "autopus-qamesh-"+sandboxHomeDirName)
	_ = os.MkdirAll(fallback, 0o755)
	return fallback
}

// sandboxHomeContained reports whether home is safely inside the cache root.
// The symlink walk only applies when the cache root is inside the project; the
// fallback cache root is a private temp directory the harness just created.
func sandboxHomeContained(paths goCachePaths, home string) bool {
	if !strings.HasPrefix(paths.Root, paths.ProjectDir+string(filepath.Separator)) {
		return true
	}
	return validateNoSymlinkComponents(paths.ProjectDir, home) == nil
}
