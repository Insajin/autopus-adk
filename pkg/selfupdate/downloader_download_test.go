package selfupdate

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDownloadAndVerify_Success verifies that a valid tar.gz archive with a
// matching checksum is downloaded and extracted successfully.
// R2: download release archive. R3: extract binary. R6: verify SHA256 checksum.
func TestDownloadAndVerify_Success(t *testing.T) {
	t.Parallel()

	archiveContent := buildTarGz(t, "auto", "#!/bin/sh\necho hello")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"
	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	envelope := releaseSignatureEnvelope(t, []byte(checksumLine),
		testEnvelopeSigner{private: priv, fingerprint: pinned.Fingerprint})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "autopus-adk-selfupdate", r.Header.Get("User-Agent"))

		switch r.URL.Path {
		case "/" + archiveName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(archiveContent)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksumLine))
		case "/checksums.txt.signatures":
			_, _ = w.Write(envelope)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := newDownloaderForTest([]pinnedReleaseKey{pinned}, referenceTime)
	binaryPath, err := dl.DownloadAndVerify(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		archiveName,
		destDir,
	)

	require.NoError(t, err)
	assert.NotEmpty(t, binaryPath)
}

func TestDownloadAndVerify_CompatibilityMethodDerivesSignatureURLStrictly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		checksumURL string
		want        string
		wantError   bool
	}{
		{
			name:        "release asset",
			checksumURL: "https://github.com/Insajin/autopus-adk/releases/download/v0.50.73/checksums.txt",
			want:        "https://github.com/Insajin/autopus-adk/releases/download/v0.50.73/checksums.txt.signatures",
		},
		{
			name:        "query preserved",
			checksumURL: "https://example.test/release/checksums.txt?download=1",
			want:        "https://example.test/release/checksums.txt.signatures?download=1",
		},
		{name: "wrong basename", checksumURL: "https://example.test/release/sums.txt", wantError: true},
		{name: "encoded basename", checksumURL: "https://example.test/release/checksums%2Etxt", wantError: true},
		{name: "userinfo", checksumURL: "https://user@example.test/release/checksums.txt", wantError: true},
		{name: "fragment", checksumURL: "https://example.test/release/checksums.txt#asset", wantError: true},
		{name: "unsupported scheme", checksumURL: "file:///tmp/checksums.txt", wantError: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := deriveReleaseSignaturesURL(test.checksumURL)

			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestDownloadAndVerifyWithSignature_MissingURLFailsBeforeNetwork(t *testing.T) {
	t.Parallel()

	dl := NewDownloader()
	_, err := dl.DownloadAndVerifyWithSignature(
		"https://example.test/archive.tar.gz",
		"https://example.test/checksums.txt",
		"",
		"archive.tar.gz",
		t.TempDir(),
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "release signatures")
}

func TestDownloadAndVerifyWithSignature_InvalidOrUntrustedEnvelopeSkipsArchive(t *testing.T) {
	t.Parallel()

	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := strings.Repeat("0", 64) + "  " + archiveName + "\n"
	_, trusted := generateReleaseTestKey(t, "2099-12-31")
	attackerPrivate, attacker := generateReleaseTestKey(t, "2099-12-31")
	attackerEnvelope := releaseSignatureEnvelope(t, []byte(checksumLine), testEnvelopeSigner{
		private:     attackerPrivate,
		fingerprint: attacker.Fingerprint,
	})

	for _, test := range []struct {
		name     string
		envelope []byte
	}{
		{name: "malformed", envelope: []byte("not-an-envelope\n")},
		{name: "untrusted signer", envelope: attackerEnvelope},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var archiveRequests atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/checksums.txt":
					_, _ = w.Write([]byte(checksumLine))
				case "/checksums.txt.signatures":
					_, _ = w.Write(test.envelope)
				case "/" + archiveName:
					archiveRequests.Add(1)
					_, _ = w.Write([]byte("must not be downloaded"))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			dl := newDownloaderForTest([]pinnedReleaseKey{trusted}, referenceTime)
			_, err := dl.DownloadAndVerifyWithSignature(
				srv.URL+"/"+archiveName,
				srv.URL+"/checksums.txt",
				srv.URL+"/checksums.txt.signatures",
				archiveName,
				t.TempDir(),
			)

			require.Error(t, err)
			require.Zero(t, archiveRequests.Load())
		})
	}
}

func TestHTTPGetWithRetry_RejectsOversizedResponseWithoutTruncation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 33)))
	}))
	defer srv.Close()

	_, err := httpGetWithRetry(srv.URL, 32)

	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds")
}

// TestDownloadAndVerify_HTTPError verifies that non-200 HTTP responses
// are detected and retried instead of silently hashing HTML error pages.
func TestDownloadAndVerify_HTTPError(t *testing.T) {
	t.Parallel()

	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>rate limited</html>"))
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := NewDownloader()
	_, err := dl.DownloadAndVerifyWithSignature(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.signatures",
		archiveName,
		destDir,
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
}

func TestGitHubTokenForURL(t *testing.T) {
	t.Setenv("AUTOPUS_GITHUB_TOKEN", "env-token")

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"github", "https://github.com/insajin/autopus-adk/releases/download/v0.7.0/archive.zip", "env-token"},
		{"api github", "https://api.github.com/repos/insajin/autopus-adk/releases/latest", "env-token"},
		{"github subdomain", "https://uploads.github.com/repos/insajin/autopus-adk/releases/assets/1", "env-token"},
		{"githubusercontent cdn", "https://objects.githubusercontent.com/github-production-release-asset/example", ""},
		{"non github", "https://example.com/archive.zip", ""},
		{"invalid", "://bad-url", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, githubTokenForURL(tt.url))
		})
	}
}

// TestDownloadAndVerify_RetrySuccess verifies that transient HTTP errors
// are retried and succeed on subsequent attempts.
func TestDownloadAndVerify_RetrySuccess(t *testing.T) {
	t.Parallel()

	archiveContent := buildTarGz(t, "auto", "#!/bin/sh\necho hello")
	checksum := fmt.Sprintf("%x", sha256.Sum256(archiveContent))
	archiveName := "autopus-adk_0.7.0_darwin_arm64.tar.gz"
	checksumLine := checksum + "  " + archiveName + "\n"
	priv, pinned := generateReleaseTestKey(t, "2099-12-31")
	envelope := releaseSignatureEnvelope(t, []byte(checksumLine),
		testEnvelopeSigner{private: priv, fingerprint: pinned.Fingerprint})

	var archiveRequests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/"+archiveName && archiveRequests.Add(1) <= 2 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		switch r.URL.Path {
		case "/" + archiveName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(archiveContent)
		case "/checksums.txt":
			_, _ = w.Write([]byte(checksumLine))
		case "/checksums.txt.signatures":
			_, _ = w.Write(envelope)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	destDir := t.TempDir()
	dl := newDownloaderForTest([]pinnedReleaseKey{pinned}, referenceTime)
	binaryPath, err := dl.DownloadAndVerifyWithSignature(
		srv.URL+"/"+archiveName,
		srv.URL+"/checksums.txt",
		srv.URL+"/checksums.txt.signatures",
		archiveName,
		destDir,
	)

	require.NoError(t, err)
	assert.NotEmpty(t, binaryPath)
}
