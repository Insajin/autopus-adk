package companionmanifest

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 릴리즈 계보 계약의 단일 표다. produce-public-key-receipt.sh의 좌표 표와 1:1로 대응하며,
// 새 릴리즈는 그 표에 한 행을, 여기에 한 항목을, 필요하면 releasePhasePriorPins에 한 항목을
// 더하면 된다 — 좌표마다 새 테스트 파일을 만들지 않는다.
type releasePhase struct {
	phase   string
	tag     string
	version string

	// validate-source.sh 성공 출력에서 함께 확인할 필드. 빈 값이면 이 좌표에는 전용 accept
	// 검증이 없다 — A0..A4는 release_lineage_test.go가 직접 덮는다.
	acceptedField string
	// "identity"는 lightweight 태그 / 직전 릴리즈 부재 / 미승인 소스 세 가지를 거부하는지,
	// "unsignedTag"는 서명이 필수인 상태에서 서명 없는 태그를 거부하는지 검증한다.
	rejects string
	// validate-source.sh가 정확히 한 번 선언해야 하는 직전 immutable 릴리즈 ancestry 핀.
	ancestorSHA string
	// 이 좌표만 요구하는 추가 source gate 문자열.
	extraSourceGates []string

	// verify-public-key-lineage-coordinates.sh가 담아야 하는 좌표 열.
	pinsRepository     bool
	pinsEvidenceSource bool
	pinsTagObject      bool
	pinsLinuxArchives  bool
	pinsReleaseID      bool
	// verify-public-key-lineage.sh 호출자가 유지해야 하는 계약.
	callerTreeSHA      bool
	callerAssetsHelper bool
	callerReleaseID    bool
	// verify-public-key-lineage-assets.sh의 네 아카이브 / 두 Darwin 번들 계약.
	assetContract bool
	// A0..A13에는 Linux 아카이브 핀이 존재해서는 안 된다.
	forbidsHistoricalLinuxPins bool
	// The A22 bridge predecessor has a dedicated immutable asset proof path.
	bridgePredecessor bool
}

// priorPhase는 직전 좌표의 phase 이름이다.
func (p releasePhase) priorPhase(t *testing.T) string {
	t.Helper()
	index, err := strconv.Atoi(strings.TrimPrefix(p.phase, "A"))
	if err != nil || index < 1 {
		t.Fatalf("phase %q has no direct predecessor", p.phase)
	}
	return "A" + strconv.Itoa(index-1)
}

var releasePhases = []releasePhase{
	{
		phase: "A0", tag: "v0.50.69", version: "0.50.69",
	},
	{
		phase: "A1", tag: "v0.50.70", version: "0.50.70",
	},
	{
		phase: "A2", tag: "v0.50.71", version: "0.50.71",
	},
	{
		phase: "A3", tag: "v0.50.72", version: "0.50.72",
	},
	{
		phase: "A4", tag: "v0.50.73", version: "0.50.73",
	},
	{
		phase: "A5", tag: "v0.50.74", version: "0.50.74",
		acceptedField: "source-commit",
		rejects:       "identity",
	},
	{
		phase: "A6", tag: "v0.50.77", version: "0.50.77",
		acceptedField: "source-commit",
		rejects:       "identity",
	},
	{
		phase: "A7", tag: "v0.50.78", version: "0.50.78",
		acceptedField: "source-commit",
		rejects:       "identity",
	},
	{
		phase: "A8", tag: "v0.50.79", version: "0.50.79",
		acceptedField: "source-commit",
		rejects:       "identity",
		callerTreeSHA: true,
	},
	{
		phase: "A9", tag: "v0.50.80", version: "0.50.80",
		acceptedField: "source-commit",
		rejects:       "identity",
		callerTreeSHA: true,
	},
	{
		phase: "A10", tag: "v0.50.81", version: "0.50.81",
		acceptedField:      "source-commit",
		rejects:            "identity",
		ancestorSHA:        "c9c4f49d48022eb0c8d72ee7b520136a4f21f176",
		pinsEvidenceSource: true, pinsTagObject: true, callerTreeSHA: true,
	},
	{
		phase: "A11", tag: "v0.50.82", version: "0.50.82",
		acceptedField:      "source-commit",
		rejects:            "identity",
		ancestorSHA:        "54536edc09c37a634532c2c9b51e62869d393db4",
		pinsEvidenceSource: true, pinsTagObject: true, callerTreeSHA: true,
	},
	{
		phase: "A12", tag: "v0.50.83", version: "0.50.83",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9",
		pinsRepository: true, pinsEvidenceSource: true, pinsTagObject: true,
		callerTreeSHA: true,
	},
	{
		phase: "A13", tag: "v0.50.84", version: "0.50.84",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "e6367b5375cd4cdf09cb1515877bc57323521364",
		pinsRepository: true, pinsEvidenceSource: true, pinsTagObject: true,
		callerTreeSHA: true,
	},
	{
		phase: "A14", tag: "v0.50.85", version: "0.50.85",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "2b7aa046bdb7861113dfa57b30489c11715582e9",
		pinsRepository: true, pinsEvidenceSource: true, pinsTagObject: true,
		callerTreeSHA: true,
	},
	{
		phase: "A15", tag: "v0.50.86", version: "0.50.86",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "4b8eb62200d253b46e022670c482e2f716a992a3",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		callerAssetsHelper: true, assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A16", tag: "v0.50.87", version: "0.50.87",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "0fc4f60dac8ff8afe69b680c8bf723bfbced4769",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		callerAssetsHelper: true, assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A17", tag: "v0.50.88", version: "0.50.88",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "3e02c622af97f74873325ec65940c580e23c580a",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		callerAssetsHelper: true, assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A18", tag: "v0.50.89", version: "0.50.89",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "2b062a5e348fbecc414abe9ba5c74c7dc79fe243",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		pinsReleaseID: true, callerAssetsHelper: true, callerReleaseID: true,
		assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A19", tag: "v0.50.90", version: "0.50.90",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "76f35d990e76511d169e239547d33bfedcea7948",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		pinsReleaseID: true, callerAssetsHelper: true, callerReleaseID: true,
		assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A20", tag: "v0.50.91", version: "0.50.91",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "5bc41dccc72f8244943fd9e862cba07a36bf09d3",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		pinsReleaseID: true, callerAssetsHelper: true, callerReleaseID: true,
		assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A21", tag: "v0.50.92", version: "0.50.92",
		acceptedField:  "source-commit",
		rejects:        "identity",
		ancestorSHA:    "7f44e4f143b2348c02553bab2209088c966f81ae",
		pinsRepository: true, pinsEvidenceSource: true, pinsLinuxArchives: true,
		pinsReleaseID: true, callerAssetsHelper: true, callerReleaseID: true,
		assetContract: true, forbidsHistoricalLinuxPins: true,
	},
	{
		phase: "A22", tag: "v0.50.109", version: "0.50.109",
		acceptedField: "source-tree",
		rejects:       "unsignedTag",
		ancestorSHA:   "b86fab067599f457261287552c5a9dd86460d7f4",
		extraSourceGates: []string{
			"COMPANION_RELEASE_TAG_SIGNATURE_REQUIRED",
			"ADK_KEY_ROTATION_VERIFIED",
			"release-tag-signing-2026-q3-r2.pub",
			`verify-tag "refs/tags/$GITHUB_REF_NAME"`,
		},
		pinsRepository: true, pinsLinuxArchives: true, pinsReleaseID: true,
	},
	{
		phase: "A23", tag: "v0.50.111", version: "0.50.111",
		acceptedField: "source-tree",
		rejects:       "unsignedTag",
		ancestorSHA:   "67f3def5d4a0a11aadd9e103389de6cc1cafc34e",
		extraSourceGates: []string{
			"COMPANION_RELEASE_TAG_SIGNATURE_REQUIRED",
			"release-tag-signing-2026-q3-r2.pub",
			"SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ",
			`verify-tag "refs/tags/$GITHUB_REF_NAME"`,
		},
		pinsRepository: true, pinsEvidenceSource: true, pinsTagObject: true,
		pinsReleaseID: true, callerTreeSHA: true, callerReleaseID: true,
		bridgePredecessor: true,
	},
	{
		// v0.50.112 is absent on purpose. It was armed as A24, tagged, and pushed,
		// but CI failed at the tagged commit so the release job never ran and the
		// coordinate was burned. A burned coordinate never enters this table, which
		// is why v0.50.110 and v0.50.75..76 are absent too. A24 is v0.50.113.
		phase: "A24", tag: "v0.50.113", version: "0.50.113",
		acceptedField: "source-tree",
		rejects:       "unsignedTag",
		ancestorSHA:   "954f60a77acb59fd4106537020693fdcadb3d640",
		extraSourceGates: []string{
			"COMPANION_RELEASE_TAG_SIGNATURE_REQUIRED",
			"release-tag-signing-2026-q3-r2.pub",
			"SHA256:7FISPXCi8p7cFEdh4Fcyyp8RPQbXYZwmo3Mxi5+YjrQ",
			`verify-tag "refs/tags/$GITHUB_REF_NAME"`,
		},
		pinsRepository: true, pinsEvidenceSource: true, pinsTagObject: true,
		pinsReleaseID: true, callerTreeSHA: true, callerReleaseID: true,
		bridgePredecessor: true,
	},
}

const releaseCoordinateTableScript = "scripts/companion-release/produce-public-key-receipt.sh"

// 좌표는 두 곳에만 산다: 프로듀서의 bash 표와 위 releasePhases. 이 테스트가 둘을 한 원본으로 묶는다.
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
