package companionmanifest

import (
	"fmt"
	"regexp"
	"testing"
)

// 무장된 좌표는 네 곳에 있고, 그 중 셋은 줄일 수 없다.
//
//   - release.yaml의 push 태그 트리거: GitHub Actions는 트리거를 동적으로 만들 수
//     없고, release_contract_test는 `v*` 와일드카드를 금지한다(임의 태그가 보호된
//     릴리스 잡에 들어오는 것을 막기 위해).
//   - homebrew 복구 워크플로의 `if` 가드: workflow_dispatch에는 태그 필터가 없어서,
//     이 비교만이 이미 게시된 옛 좌표로 재실행해 tap Cask를 되돌리는 것을 막는다.
//   - prepare-release.sh의 release_tag: 사람이 릴리스를 준비할 때 무장하는 지점이다.
//
// 네 번째는 프로듀서의 좌표 표이며 데이터다. 이 테스트는 그 넷이 서로 어긋나는
// 순간을 실패로 만든다. 어긋남은 예전에는 태그를 밀고 CI가 절반쯤 진행한 뒤에야
// 드러났고, 그 시점의 실패는 원인과 무관한 이름을 달고 나왔다.
func TestArmedReleaseCoordinateAgreesEverywhere(t *testing.T) {
	expected := releasePhases[len(releasePhases)-1].tag
	if expected == "" {
		t.Fatal("declared release phases carry no armed tag")
	}

	for _, site := range []struct {
		name    string
		file    string
		pattern string
	}{
		{
			name:    "release workflow push trigger",
			file:    ".github/workflows/release.yaml",
			pattern: `(?m)^\s*-\s*'(v[0-9][^']*)'\s*$`,
		},
		{
			name:    "homebrew recovery ref guard",
			file:    ".github/workflows/homebrew-formula-bridge-recovery.yaml",
			pattern: `github\.ref == 'refs/tags/(v[0-9][^']*)'`,
		},
		{
			name:    "prepare-release armed tag",
			file:    "scripts/companion-release/prepare-release.sh",
			pattern: `readonly release_tag='(v[0-9][^']*)'`,
		},
	} {
		t.Run(site.name, func(t *testing.T) {
			body := readReleaseFile(t, site.file)
			match := regexp.MustCompile(site.pattern).FindStringSubmatch(body)
			if match == nil {
				t.Fatalf("%s no longer states an armed coordinate matching %s",
					site.file, site.pattern)
			}
			if match[1] != expected {
				t.Fatalf("%s arms %s while the declared phase table arms %s — "+
					"every armed site must move together",
					site.file, match[1], expected)
			}
		})
	}
}

// 좌표를 올릴 때 무엇을 만져야 하는지 실패 메시지로 남긴다. 목록이 아니라 테스트가
// 진실이어야 하므로, 위 테스트가 참조하는 파일 집합을 그대로 되풀이해 센다.
func TestArmedReleaseCoordinateSiteCountIsBounded(t *testing.T) {
	const bounded = 4
	sites := []string{
		".github/workflows/release.yaml",
		".github/workflows/homebrew-formula-bridge-recovery.yaml",
		"scripts/companion-release/prepare-release.sh",
		releaseCoordinateTableScript,
	}
	if len(sites) != bounded {
		t.Fatalf("armed coordinate now lives in %d places, not %d: %v",
			len(sites), bounded, sites)
	}
	armed := regexp.MustCompile(`v0\.50\.\d+|0\.50\.\d+`)
	for _, site := range sites {
		if !armed.MatchString(readReleaseFile(t, site)) {
			t.Fatal(fmt.Sprintf("%s stopped naming a coordinate; "+
				"if it became derived, drop it from this list", site))
		}
	}
}
