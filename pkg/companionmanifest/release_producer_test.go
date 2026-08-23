package companionmanifest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompanionReleaseProducer_DarwinTargets_UseExactAssociatedFiles(t *testing.T) {
	for _, architecture := range []string{"amd64", "arm64"} {
		t.Run(architecture, func(t *testing.T) {
			dir := t.TempDir()
			artifactDir := filepath.Join(dir, "auto_darwin_"+architecture)
			if err := os.Mkdir(artifactDir, 0o700); err != nil {
				t.Fatal(err)
			}
			artifact := filepath.Join(artifactDir, "auto")
			if err := os.WriteFile(artifact, []byte("artifact-"+architecture), 0o700); err != nil {
				t.Fatal(err)
			}
			secret := "private-release-material-" + architecture
			keyFile := filepath.Join(dir, "release-key")
			if err := os.WriteFile(keyFile, []byte(secret), 0o600); err != nil {
				t.Fatal(err)
			}
			argsFile := filepath.Join(dir, "signer-args")
			stdinDigestFile := filepath.Join(dir, "stdin-digest")
			signer := writeSignerWrapper(t, dir)
			command := exec.Command("bash", releaseProducerPath(t))
			command.Env = append(companionProducerEnv(
				artifact, architecture, keyFile, signer, argsFile, stdinDigestFile,
			), darwinReleaseToolEnv(t, dir)...)
			command.Env = append(command.Env, releaseCanaryGateEnv(t, dir, architecture)...)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("producer failed: %v\n%s", err, output)
			}
			if strings.Contains(string(output), secret) {
				t.Fatal("producer output leaked private key")
			}
			assertProducerOutputs(t, artifactDir, architecture)
			assertSignerTransport(t, argsFile, stdinDigestFile, artifact, secret)
			assertReleaseCanaryExecuted(t, dir, architecture == "arm64")
		})
	}
}
func TestCompanionReleaseProducer_MissingKeyID_FailsWithoutSecretDisclosure(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "auto")
	if err := os.WriteFile(artifact, []byte("artifact"), 0o700); err != nil {
		t.Fatal(err)
	}
	secret := "private-release-material"
	keyFile := filepath.Join(dir, "release-key")
	if err := os.WriteFile(keyFile, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	environment := companionProducerEnv(artifact, "arm64", keyFile, writeSignerWrapper(t, dir),
		filepath.Join(dir, "args"), filepath.Join(dir, "digest"))
	environment = removeEnvironment(environment, "COMPANION_KEY_ID")
	command := exec.Command("bash", releaseProducerPath(t))
	command.Env = append(environment, darwinReleaseToolEnv(t, dir)...)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("producer succeeded without key ID")
	}
	if strings.Contains(string(output), secret) {
		t.Fatal("producer failure leaked private key")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "adk-companion-manifest.json")); !os.IsNotExist(statErr) {
		t.Fatalf("manifest exists after rejected input: %v", statErr)
	}
}
func companionProducerEnv(artifact, architecture, keyFile, signer, argsFile, digestFile string) []string {
	environment := append(os.Environ(),
		"GO_WANT_COMPANION_SIGNER_HELPER=1",
		"FAKE_ARGS_OUT="+argsFile,
		"FAKE_STDIN_DIGEST="+digestFile,
		"COMPANION_ARTIFACT="+artifact,
		"COMPANION_PLATFORM=darwin",
		"COMPANION_ARCHITECTURE="+architecture,
		"COMPANION_TARGET=darwin_"+architecture,
		"COMPANION_VERSION=0.50.69",
		"COMPANION_BUILD_PROVENANCE=github-actions:Insajin/autopus-adk@abcdef",
		"COMPANION_HANDOFF=v1",
		"COMPANION_ROLLBACK_FLOOR=5069",
		"COMPANION_ISSUED_AT=2026-07-14T00:00:00Z",
		"COMPANION_EXPIRES_AT=2026-07-21T00:00:00Z",
		"COMPANION_KEY_ID=release-2026-q3",
		"COMPANION_SIGNING_KEY_FILE="+keyFile,
		"COMPANION_SIGNER="+signer,
	)
	if architecture == "arm64" {
		root := filepath.Join(filepath.Dir(keyFile), "omp-release-canary-root")
		environment = append(environment,
			// lineage 쌍은 좌표 무장이 사라진 뒤 모든 darwin/arm64 릴리즈가
			// 내보내는 자산이 됐다. 이 하네스도 그 경로를 지나므로 입력을
			// 준다 - internal/companionmanifest의 mocked 릴리즈와 같은 이유다.
			"OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256="+strings.Repeat("0", 64),
			"COMPANION_SOURCE_COMMIT="+strings.Repeat("1", 40),
			"COMPANION_SOURCE_TREE="+strings.Repeat("2", 40),
			"OMP_CONTEXT_RELEASE_CANARY_ROOT="+root,
			"OMP_CONTEXT_RELEASE_CANARY_EXECUTABLE="+filepath.Join(root, "omp-darwin-arm64"))
	}
	return environment
}

func writeSignerWrapper(t *testing.T, dir string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "signer")
	// 래퍼가 활성 플래그를 직접 들고 간다. producer는 lineage 서명만 `env -i`
	// 로 격리 호출하므로, 상속에 의존하면 그 호출에서 헬퍼가 잠들어 아무
	// 산출물도 남기지 않는다.
	script := "#!/usr/bin/env bash\nexec env GO_WANT_COMPANION_SIGNER_HELPER=1 \"" +
		executable + "\" -test.run=TestCompanionSignerHelperProcess -- \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	writeReleaseCanaryFixture(t, dir)
	return path
}
func releaseProducerPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", "companion-release", "produce.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func assertProducerOutputs(t *testing.T, dir, architecture string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"adk-companion-darwin-receipt.json",
		"adk-companion-manifest.json",
		"adk-companion-manifest.sig",
		"auto",
	}
	// lineage 쌍은 darwin/arm64 전용이고, 좌표 무장이 사라진 뒤로는 그 타깃의
	// 모든 릴리즈가 반드시 내보낸다. autopus-desktop의 아카이브 allowlist가
	// 정확히 11개를 요구하므로 누락은 소비자 거절로 이어진다. 여기서 아키텍처를
	// 구분해 그 불변식을 golden 에 박는다. ReadDir는 이름 정렬이므로 뒤에 온다.
	if architecture == "arm64" {
		want = append(want, "release-lineage-v1.json", "release-lineage-v1.sig")
	}
	if len(entries) != len(want) {
		t.Fatalf("artifact directory entries = %v", entries)
	}
	for index, entry := range entries {
		if entry.Name() != want[index] {
			t.Fatalf("entry[%d] = %q, want %q", index, entry.Name(), want[index])
		}
	}
	signature, err := os.ReadFile(filepath.Join(dir, "adk-companion-manifest.sig"))
	if err != nil || len(signature) != 64 {
		t.Fatalf("%s signature length = %d, error = %v", architecture, len(signature), err)
	}
}
func assertSignerTransport(t *testing.T, argsFile, digestFile, artifact, secret string) {
	t.Helper()
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), secret) {
		t.Fatal("private key was passed through argv")
	}
	wantArgs := "companion-manifest\x00sign\x00--artifact\x00" + artifact
	if !strings.HasPrefix(string(args), wantArgs) {
		t.Fatalf("signer args = %q", args)
	}
	digest, err := os.ReadFile(digestFile)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256([]byte(secret))
	if string(digest) != hex.EncodeToString(wantDigest[:]) {
		t.Fatal("signer did not receive key bytes through stdin")
	}
}

// removeEnvironment는 상속 환경에서 이름 하나를 걷어낸다.
func removeEnvironment(environment []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		if !strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
