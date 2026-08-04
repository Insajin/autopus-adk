//go:build darwin || linux

package promptlayer

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const ompContextPromotionExecutableHelperEnvV3 = "AUTOPUS_OMP_CONTEXT_EXECUTABLE_HELPER_V3"

func TestCurrentOMPContextPromotionExecutableSHA256V3_Unchanged_MatchesCurrentImage(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	got, err := currentOMPContextPromotionExecutableSHA256V3()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("current executable digest mismatch: got %q want %q", got, want)
	}
}

func TestCurrentOMPContextPromotionExecutableSHA256V3_LaunchThenReplace_DoesNotTrustReplacementPath(t *testing.T) {
	if os.Getenv(ompContextPromotionExecutableHelperEnvV3) == "1" {
		runOMPContextPromotionExecutableHelperV3()
		return
	}
	t.Parallel()

	tmp := t.TempDir()
	runner := filepath.Join(tmp, "promotion-runtime")
	ready := filepath.Join(tmp, "ready")
	gate := filepath.Join(tmp, "gate")
	result := filepath.Join(tmp, "result")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runner, original, 0o555); err != nil {
		t.Fatal(err)
	}
	expected := fmt.Sprintf("sha256:%x", sha256.Sum256(original))
	cmd := exec.Command(runner, "-test.run=TestCurrentOMPContextPromotionExecutableSHA256V3_LaunchThenReplace_DoesNotTrustReplacementPath")
	cmd.Env = append(os.Environ(),
		ompContextPromotionExecutableHelperEnvV3+"=1",
		"AUTOPUS_OMP_CONTEXT_EXECUTABLE_READY_V3="+ready,
		"AUTOPUS_OMP_CONTEXT_EXECUTABLE_GATE_V3="+gate,
		"AUTOPUS_OMP_CONTEXT_EXECUTABLE_RESULT_V3="+result,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForOMPContextPromotionExecutableFileV3(t, ready)

	replacement := filepath.Join(tmp, "replacement")
	replacementBody := []byte("replacement executable image")
	if err := os.WriteFile(replacement, replacementBody, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, runner); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gate, []byte("continue"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper failed: %v", err)
	}
	got, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	measured := strings.TrimSpace(string(got))
	replacementDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(replacementBody))
	if measured == replacementDigest {
		t.Fatalf("trusted replacement path digest %q", measured)
	}
	if runtime.GOOS == "linux" && measured != expected {
		t.Fatalf("did not hash live executable inode: got %q want %q", measured, expected)
	}
	if runtime.GOOS == "darwin" && measured != "error: OMP context promotion runtime executable is not the live process image" {
		t.Fatalf("did not fail closed on replaced executable path: got %q", measured)
	}
}

func runOMPContextPromotionExecutableHelperV3() {
	ready := os.Getenv("AUTOPUS_OMP_CONTEXT_EXECUTABLE_READY_V3")
	gate := os.Getenv("AUTOPUS_OMP_CONTEXT_EXECUTABLE_GATE_V3")
	result := os.Getenv("AUTOPUS_OMP_CONTEXT_EXECUTABLE_RESULT_V3")
	if ready == "" || gate == "" || result == "" {
		os.Exit(2)
	}
	if err := os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
		os.Exit(2)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(gate); err == nil {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(2)
		}
		time.Sleep(10 * time.Millisecond)
	}
	digest, err := currentOMPContextPromotionExecutableSHA256V3()
	if err != nil {
		_ = os.WriteFile(result, []byte("error: "+err.Error()), 0o600)
		return
	}
	_ = os.WriteFile(result, []byte(digest), 0o600)
}

func waitForOMPContextPromotionExecutableFileV3(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
