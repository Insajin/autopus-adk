package companionmanifest

import (
	"regexp"
	"strings"
	"testing"
)

const releaseCoordinateTableScript = "scripts/companion-release/produce-public-key-receipt.sh"

// 좌표는 두 곳에만 산다: 프로듀서의 bash 표와 releasePhases. 이 테스트가 둘을 한 원본으로 묶는다.
func TestReleaseCoordinateTable_ProducerRowsMatchDeclaredPhases(t *testing.T) {
	helper := readReleaseFile(t, releaseCoordinateTableScript)
	table := regexp.MustCompile(`(?s)readonly PUBLIC_KEY_RECEIPT_RELEASE_COORDINATES='\n(.*?)\n'`).
		FindStringSubmatch(helper)
	if table == nil {
		t.Fatal("producer receipt helper no longer declares one coordinate table")
	}
	var produced []string
	for _, line := range strings.Split(table[1], "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			produced = append(produced, strings.Join(fields, " "))
		}
	}
	declared := make([]string, 0, len(releasePhases))
	for _, phase := range releasePhases {
		declared = append(declared, phase.tag+" "+phase.version+" "+phase.phase)
	}
	if strings.Join(produced, "\n") != strings.Join(declared, "\n") {
		t.Fatalf("coordinate table drifted\nproducer:\n%s\ndeclared:\n%s",
			strings.Join(produced, "\n"), strings.Join(declared, "\n"))
	}
	// 알 수 없는 (tag, version) 조합은 여전히 정확히 이 코드로 크게 실패해야 한다.
	for _, required := range []string{
		`fail 'public_key_receipt_release_identity_mismatch'`,
		`[[ "$GITHUB_REF_NAME" == "$coordinate_tag" && "$COMPANION_VERSION" == "$coordinate_version" ]]`,
	} {
		if !strings.Contains(helper, required) {
			t.Fatalf("coordinate resolver no longer enforces %q", required)
		}
	}
}
