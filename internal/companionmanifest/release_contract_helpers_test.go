package companionmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// releaseWorkflowLineAt returns the source line containing offset.
func releaseWorkflowLineAt(source string, offset int) string {
	start := offset
	for start > 0 && source[start-1] != '\n' {
		start--
	}
	end := offset
	for end < len(source) && source[end] != '\n' {
		end++
	}
	return source[start:end]
}

func assertHomebrewOpenSSL3Selection(t *testing.T, run string, env map[string]any) {
	t.Helper()
	const want = `set -euo pipefail
openssl_prefix=$(brew --prefix openssl@3)
[[ -n "$openssl_prefix" ]]
openssl_bin="$openssl_prefix/bin"
[[ -x "$openssl_bin/openssl" ]]
openssl_version=$("$openssl_bin/openssl" version)
[[ "$openssl_version" == "OpenSSL 3."* ]]
printf '%s\n' "$openssl_bin" >> "$GITHUB_PATH"`
	if strings.TrimSpace(run) != want {
		t.Fatalf("OpenSSL selection must fail closed on the Homebrew openssl@3 executable:\n%s", run)
	}
	if len(env) != 0 {
		t.Fatalf("OpenSSL selection receives step credentials or environment: %#v", env)
	}
}

func readReleaseFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
