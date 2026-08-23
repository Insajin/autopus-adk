package companionmanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

// 가짜 서명자 헬퍼와 인자 조회. producer는 이 헬퍼를 manifest 서명과
// omp-context-release-lineage 두 서브커맨드로 부르며, 후자는 `env -i` 로
// 격리되므로 활성 플래그를 래퍼가 직접 들고 간다.
func TestCompanionSignerHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COMPANION_SIGNER_HELPER") != "1" {
		return
	}
	args := helperArguments(os.Args)
	// 좌표 무장이 사라진 뒤 lineage 쌍은 모든 darwin/arm64 릴리즈가 내보내는
	// 자산이 됐고, 그래서 producer가 이 가짜 서명자를 manifest 서명 말고
	// omp-context-release-lineage 로도 호출한다. 그 호출은 `env -i` 로 격리돼
	// FAKE_* 경로가 전달되지 않으므로, 기록 단계보다 먼저 갈라져야 한다.
	// 이 하네스의 계약은 "producer가 서명자를 무엇으로 불렀는가"이고 lineage
	// 내용은 계약이 아니므로, 요청된 두 산출물만 만들어 주고 끝낸다.
	if lineage := flagValueOrEmpty(args, "--lineage-output"); lineage != "" {
		signature := flagValue(t, args, "--signature-output")
		for path, body := range map[string]string{
			lineage: `{"schema_version":"fake.omp-context-release-lineage.v1"}` + "\n",
			// producer는 서명이 원시 Ed25519 바이트인지 길이로만 검사한다
			// (produce-omp-context-lineage.sh:50). 내용은 계약이 아니므로
			// 정확히 64바이트를 쓴다.
			signature: strings.Repeat("s", 64),
		} {
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(input)
	if err := os.WriteFile(os.Getenv("FAKE_STDIN_DIGEST"), []byte(hex.EncodeToString(sum[:])), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("FAKE_ARGS_OUT"), []byte(strings.Join(args, "\x00")), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := flagValue(t, args, "--manifest-output")
	signature := flagValue(t, args, "--signature-output")
	artifact := flagValue(t, args, "--artifact")
	artifactBytes, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	artifactSum := sha256.Sum256(artifactBytes)
	rollbackFloor, err := strconv.ParseUint(flagValue(t, args, "--rollback-floor"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := CanonicalBytes(Manifest{
		SchemaVersion: SchemaVersion, ArtifactDigest: "sha256:" + hex.EncodeToString(artifactSum[:]),
		Version: flagValue(t, args, "--version"), Platform: flagValue(t, args, "--platform"),
		Architecture:    flagValue(t, args, "--architecture"),
		BuildProvenance: flagValue(t, args, "--build-provenance"),
		Handoff:         flagValue(t, args, "--handoff"), RollbackFloor: rollbackFloor,
		IssuedAt: flagValue(t, args, "--issued-at"), ExpiresAt: flagValue(t, args, "--expires-at"),
		KeyID: flagValue(t, args, "--key-id"),
	})
	if err != nil {
		t.Fatal(err)
	}
	appendDarwinReleaseEvent(t, "manifest_signature")
	if err := os.WriteFile(manifest, manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(signature, bytes.Repeat([]byte{0x5a}, 64), 0o600); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FAKE_POST_MANIFEST_MUTATION") == "1" {
		if err := os.WriteFile(artifact, append(artifactBytes, []byte("-mutated")...), 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func helperArguments(args []string) []string {
	for index, arg := range args {
		if arg == "--" {
			return args[index+1:]
		}
	}
	return nil
}
func flagValue(t *testing.T, args []string, name string) string {
	t.Helper()
	for index := 0; index < len(args)-1; index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	t.Fatalf("missing helper flag %s in %v", name, args)
	return ""
}

// flagValueOrEmpty는 flagValue와 달리 없을 때 실패하지 않는다. 서명자가 두 가지
// 서브커맨드로 불리므로, 어느 쪽인지 인자로 판별해야 한다.
func flagValueOrEmpty(args []string, name string) string {
	for index := range len(args) - 1 {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
}
