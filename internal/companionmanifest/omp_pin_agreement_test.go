package companionmanifest

import (
	"regexp"
	"testing"
)

// The OMP pin lives in seven places across five files and none of them can be
// derived from another at build time: two are shell literals the release runs
// under `env -i`, one is a download URL, one is a Go constant compiled into the
// exec smoke, and one is a label inside the active policy identity string.
//
// That is the same shape as the armed release coordinate, and it was unguarded.
// A missed site does not fail until the release lane is already running, which
// is far too late — so this test fails the moment any site disagrees, and
// `scripts/release-tools/advance-omp-pin.sh` moves them together.
//
// The pin is not what enforces the protocol contract. `initializeManaged`
// negotiates protocol v2, turns auto-retry and auto-compaction off, and refuses
// a runtime without native image compaction, all against the binary that
// actually runs. The pin's job is narrower and still worth keeping: it declares,
// before the release starts, exactly which executable the evidence will name.
type ompPinSite struct {
	name    string
	file    string
	pattern string
}

func ompPinVersionSites() []ompPinSite {
	return []ompPinSite{
		{
			name:    "prepare-release staged path",
			file:    "scripts/companion-release/prepare-release.sh",
			pattern: `staged_omp="\$temp_dir/omp-v([0-9]+\.[0-9]+\.[0-9]+)"`,
		},
		{
			name:    "prepare-release version assertion",
			file:    "scripts/companion-release/prepare-release.sh",
			pattern: `== 'omp/([0-9]+\.[0-9]+\.[0-9]+)'`,
		},
		{
			name:    "producer report omp-version flag",
			file:    "scripts/companion-release/prepare-release-local-lib.sh",
			pattern: `--omp-version 'omp/([0-9]+\.[0-9]+\.[0-9]+)'`,
		},
		{
			name:    "canary staging path",
			file:    "scripts/companion-release/materialize-omp-release-canary.sh",
			pattern: `omp-v([0-9]+\.[0-9]+\.[0-9]+)-darwin-arm64\.download`,
		},
		{
			name:    "canary download url",
			file:    "scripts/companion-release/materialize-omp-release-canary.sh",
			pattern: `releases/download/v([0-9]+\.[0-9]+\.[0-9]+)/omp-darwin-arm64`,
		},
		{
			name:    "exec smoke pinned version",
			file:    "scripts/companion-release/execsmoke/main.go",
			pattern: `pinnedOMPVersion = "omp/([0-9]+\.[0-9]+\.[0-9]+)"`,
		},
		{
			name:    "active policy image schema",
			file:    "internal/cli/pipeline_omp_context_active_process.go",
			pattern: `snapcompact-image-schema=omp-v([0-9]+\.[0-9]+\.[0-9]+)`,
		},
		{
			// The hardening test asserts the shipped materializer URL, so it is
			// an armed site rather than a fixture. It sat outside this list and
			// outside the advance script, and the first real pin move left it
			// behind — which is exactly what this guard exists to catch.
			name:    "exec smoke hardening url assertion",
			file:    "scripts/companion-release/tests/release-exec-smoke-hardening-test.sh",
			pattern: `download/v([0-9]+\.[0-9]+\.[0-9]+)/omp-darwin-arm64`,
		},
	}
}

func TestOMPPinAgreesEverywhere(t *testing.T) {
	t.Parallel()

	sites := ompPinVersionSites()
	declared := ""
	for _, site := range sites {
		body := readReleaseFile(t, site.file)
		match := regexp.MustCompile(site.pattern).FindStringSubmatch(body)
		if match == nil {
			t.Fatalf("%s (%s) no longer states an OMP version matching %s",
				site.name, site.file, site.pattern)
		}
		if declared == "" {
			declared = match[1]
			continue
		}
		if match[1] != declared {
			t.Fatalf("%s pins omp/%s while the first site pins omp/%s — "+
				"every OMP pin site must move together; run "+
				"scripts/release-tools/advance-omp-pin.sh",
				site.name, match[1], declared)
		}
	}
	if declared == "" {
		t.Fatal("no OMP pin site was readable")
	}
}

// The digest is the integrity half of the pin: the version says which release
// and this says which bytes. It is declared once and every other reader awks it
// out of prepare-release.sh, so a second literal anywhere is drift waiting to
// happen.
func TestOMPPinDigestIsDeclaredExactlyOnce(t *testing.T) {
	t.Parallel()

	body := readReleaseFile(t, "scripts/companion-release/prepare-release.sh")
	declaration := regexp.MustCompile(`(?m)^readonly expected_omp_sha256='([0-9a-f]{64})'$`).
		FindAllStringSubmatch(body, -1)
	if len(declaration) != 1 {
		t.Fatalf("prepare-release.sh declares expected_omp_sha256 %d time(s), want exactly 1",
			len(declaration))
	}
	digest := declaration[0][1]

	for _, file := range []string{
		"scripts/release-tools/preflight-prep-inputs-lib.sh",
		"scripts/release-tools/release-prep.sh",
		"scripts/companion-release/prepare-release-local-lib.sh",
		"scripts/companion-release/prepare-release-runtime-lib.sh",
	} {
		reader := readReleaseFile(t, file)
		if regexp.MustCompile(digest).MatchString(reader) {
			t.Fatalf("%s hardcodes the OMP digest instead of reading it from "+
				"prepare-release.sh; two copies drift apart", file)
		}
	}
}

// The pin must name a real upstream release shape. A version that is not three
// numeric components cannot resolve a download URL, and the failure would land
// mid-release rather than here.
func TestOMPPinNamesAResolvableUpstreamAsset(t *testing.T) {
	t.Parallel()

	body := readReleaseFile(t, "scripts/companion-release/materialize-omp-release-canary.sh")
	url := regexp.MustCompile(`https://github\.com/can1357/oh-my-pi/releases/download/v([0-9]+\.[0-9]+\.[0-9]+)/omp-darwin-arm64`).
		FindStringSubmatch(body)
	if url == nil {
		t.Fatal("the canary no longer downloads a versioned oh-my-pi darwin-arm64 asset")
	}
	staged := regexp.MustCompile(`omp-v([0-9]+\.[0-9]+\.[0-9]+)-darwin-arm64\.download`).
		FindStringSubmatch(body)
	if staged == nil || staged[1] != url[1] {
		t.Fatalf("the canary stages omp-v%v but downloads v%s",
			staged, url[1])
	}
}
