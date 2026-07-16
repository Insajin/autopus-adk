package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDarwinReleaseEnvironment_MissingTrustInputsFailClosed(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "auto")
	keyFile := filepath.Join(dir, "release-key")
	if err := os.WriteFile(artifact, []byte("auto"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("private-release-material"), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := companionProducerEnv(
		artifact, "arm64", keyFile, writeSignerWrapper(t, dir),
		filepath.Join(dir, "args"), filepath.Join(dir, "digest"),
	)
	environment = append(environment, darwinReleaseToolEnv(t, dir)...)
	for _, missing := range []string{
		"COMPANION_SIGNING_KEY_FILE", "COMPANION_SIGNER", "COMPANION_KEY_ID",
		"COMPANION_HANDOFF", "COMPANION_ROLLBACK_FLOOR", "COMPANION_ISSUED_AT",
		"COMPANION_EXPIRES_AT", "APPLE_SIGNING_IDENTITY", "APPLE_API_KEY",
		"APPLE_API_ISSUER", "APPLE_API_KEY_PATH",
	} {
		t.Run(missing, func(t *testing.T) {
			command := exec.Command("bash", releaseEnvironmentValidatorPath(t))
			command.Env = removeEnvironment(environment, missing)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("validator accepted missing %s", missing)
			}
			if strings.Contains(string(output), "private-release-material") {
				t.Fatal("validator output leaked signing material")
			}
		})
	}
}

func releaseEnvironmentValidatorPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", "companion-release", "validate-environment.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
