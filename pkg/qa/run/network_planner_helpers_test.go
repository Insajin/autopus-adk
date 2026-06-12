package run

import (
	"testing"

	qacompile "github.com/insajin/autopus-adk/pkg/qa/compile"
)

// TestRequestLabelAndOrigin covers the string, map, missing-origin, and
// default branches of requestLabelAndOrigin plus URL normalization.
func TestRequestLabelAndOrigin(t *testing.T) {
	t.Run("string url with default port", func(t *testing.T) {
		label, origin, ok := requestLabelAndOrigin("https://api.example.com:443/v1/login")
		if !ok {
			t.Fatal("expected ok for valid https url")
		}
		if origin != "https://api.example.com" {
			t.Fatalf("origin = %q, want https://api.example.com (default port stripped)", origin)
		}
		if label != "https://api.example.com/v1/login" {
			t.Fatalf("label = %q", label)
		}
	})

	t.Run("string url with non-default port", func(t *testing.T) {
		_, origin, ok := requestLabelAndOrigin("http://localhost:3000/x")
		if !ok || origin != "http://localhost:3000" {
			t.Fatalf("origin = %q ok=%v, want http://localhost:3000", origin, ok)
		}
	})

	t.Run("map with url key", func(t *testing.T) {
		label, origin, ok := requestLabelAndOrigin(map[string]any{"url": "https://h.test/p"})
		if !ok || origin != "https://h.test" || label != "https://h.test/p" {
			t.Fatalf("map url -> label=%q origin=%q ok=%v", label, origin, ok)
		}
	})

	t.Run("map without origin", func(t *testing.T) {
		label, origin, ok := requestLabelAndOrigin(map[string]any{"method": "GET"})
		if ok || origin != "" || label != "missing_origin" {
			t.Fatalf("map missing origin -> label=%q origin=%q ok=%v", label, origin, ok)
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		label, _, ok := requestLabelAndOrigin(42)
		if ok || label != "missing_origin" {
			t.Fatalf("int request -> label=%q ok=%v", label, ok)
		}
	})

	t.Run("invalid url label", func(t *testing.T) {
		label, _, ok := requestLabelAndOrigin("::not a url::")
		if ok || label != "invalid_url" {
			t.Fatalf("invalid url -> label=%q ok=%v", label, ok)
		}
	})
}

// TestGUICommandSupportsNodeGuard covers the recognized runtimes and the
// unsupported/empty branches.
func TestGUICommandSupportsNodeGuard(t *testing.T) {
	if guiCommandSupportsNodeGuard(nil) {
		t.Fatal("empty args should not support node guard")
	}
	for _, bin := range []string{"node", "npm", "npx", "pnpm", "yarn", "yarnpkg", "playwright"} {
		if !guiCommandSupportsNodeGuard([]string{"/usr/local/bin/" + bin, "test"}) {
			t.Fatalf("%s should support node guard", bin)
		}
	}
	if guiCommandSupportsNodeGuard([]string{"/bin/bash", "run.sh"}) {
		t.Fatal("bash should not support node guard")
	}
}

// TestDeferredCandidateReason covers the deferral classifications and the
// non-deferred (empty) result.
func TestDeferredCandidateReason(t *testing.T) {
	const want = "deferred to SPEC-QAMESH-003"

	if got := deferredCandidateReason(qacompile.Candidate{Adapter: "BrowserStack"}); got != want {
		t.Fatalf("deferred adapter -> %q, want %q", got, want)
	}
	if got := deferredCandidateReason(qacompile.Candidate{InputSource: "production_session"}); got != want {
		t.Fatalf("production_session -> %q, want %q", got, want)
	}
	if got := deferredCandidateReason(qacompile.Candidate{PassFailAuthority: "AI"}); got != want {
		t.Fatalf("ai authority -> %q, want %q", got, want)
	}
	if got := deferredCandidateReason(qacompile.Candidate{
		ManualOrDeferred: true,
		ErrorCode:        "needs SPEC-QAMESH-003 work",
	}); got != want {
		t.Fatalf("manual+errorcode -> %q, want %q", got, want)
	}
	if got := deferredCandidateReason(qacompile.Candidate{Adapter: "playwright"}); got != "" {
		t.Fatalf("non-deferred candidate -> %q, want empty", got)
	}
}

// TestDeferredAdapter covers the deferred-adapter set and the default branch.
func TestDeferredAdapter(t *testing.T) {
	for _, a := range []string{"browserstack", "firebase-test-lab", "maestro", "detox", "session-replay"} {
		if !deferredAdapter(a) {
			t.Fatalf("%s should be deferred", a)
		}
	}
	if deferredAdapter("playwright") {
		t.Fatal("playwright should not be deferred")
	}
}
