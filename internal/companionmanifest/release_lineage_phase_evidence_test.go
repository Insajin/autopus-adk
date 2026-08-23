package companionmanifest

import (
	"strings"
	"testing"
)

const (
	releaseLineageCallerScript     = "scripts/companion-release/verify-public-key-lineage.sh"
	releaseLineageCoordinateScript = "scripts/companion-release/verify-public-key-lineage-coordinates.sh"
	releaseLineageAssetScript      = "scripts/companion-release/verify-public-key-lineage-assets.sh"
	releaseLineageAssetBundleCount = 2
)

func TestLineageVerifier_PhasesUseExactPredecessorCoordinate(t *testing.T) {
	coordinates := readReleaseFile(t, releaseLineageCoordinateScript)
	caller := readReleaseFile(t, releaseLineageCallerScript)
	assets := readReleaseFile(t, releaseLineageAssetScript)
	for _, phase := range releasePhases {
		if _, pinned := releasePhasePriorPins[phase.phase]; !pinned {
			continue
		}
		prior := phase.priorPhase(t)
		t.Run(phase.phase, func(t *testing.T) {
			for _, want := range phase.coordinateContract(prior) {
				if !strings.Contains(coordinates, want) {
					t.Fatalf("%s lineage coordinate contract missing %q", phase.phase, want)
				}
			}
			for _, want := range phase.callerContract() {
				if !strings.Contains(caller, want) {
					t.Fatalf("%s lineage caller contract missing %q", phase.phase, want)
				}
			}
			if !phase.assetContract {
				return
			}
			for _, want := range []string{
				"_darwin_amd64.tar.gz", "_darwin_arm64.tar.gz",
				"_linux_amd64.tar.gz", "_linux_arm64.tar.gz",
				`actual_asset_digest" == "$asset_digest`,
				`actual_asset_digest" == "sha256:$archive_pin`,
				`sha256_file "$download_dir/$asset`,
				`extract_bundle "$download_dir/$darwin_amd64_asset`,
				`extract_bundle "$download_dir/$darwin_arm64_asset`,
			} {
				if !strings.Contains(assets, want) {
					t.Fatalf("%s lineage asset contract missing %q", phase.phase, want)
				}
			}
			if strings.Count(assets, "extract_bundle ") != releaseLineageAssetBundleCount {
				t.Fatal("only the two Darwin archives may be extracted as manifest bundles")
			}
		})
	}
}

// coordinateContract는 verify-public-key-lineage-coordinates.sh가 이 좌표에 대해
// 담고 있어야 하는 정확한 열들이다.
func (p releasePhase) coordinateContract(prior string) []string {
	required := []string{
		"release_phase='" + p.phase + "' prior_phase='" + prior + "'",
		`prior_tree="$` + prior + `_TREE_SHA"`,
	}
	if p.pinsRepository {
		required = append(required, prior+"_REPOSITORY='Insajin/autopus-adk' "+
			p.phase+"_TAG='"+p.tag+"' "+p.phase+"_VERSION='"+p.version+"'")
	}
	if p.pinsEvidenceSource {
		required = append(required, `prior_evidence_source='immutable `+prior+` GitHub release'`)
	}
	if p.pinsTagObject {
		required = append(required, `prior_tag_object="$`+prior+`_TAG_OBJECT_SHA" prior_checksums="$`+
			prior+`_CHECKSUMS_SHA256"`)
	}
	if p.pinsLinuxArchives {
		required = append(required,
			`prior_linux_amd64_archive="$`+prior+`_LINUX_AMD64_ARCHIVE_SHA256"`,
			`prior_linux_arm64_archive="$`+prior+`_LINUX_ARM64_ARCHIVE_SHA256"`)
	}
	if p.pinsReleaseID {
		required = append(required, `prior_release_id="$`+prior+`_RELEASE_ID"`)
	}
	return required
}

// callerContract는 verify-public-key-lineage.sh가 좌표별 증거를 위임하면서도
// 직접 유지해야 하는 검사들이다.
func (p releasePhase) callerContract() []string {
	var required []string
	if p.callerTreeSHA {
		required = append(required,
			`[[ "$(jq -er '.commit.tree.sha' "$commit_json")" == "$prior_tree" ]]`,
			`[[ -f "$coordinates_helper" && ! -L "$coordinates_helper" ]]`,
			`source "$coordinates_helper"`)
	}
	if p.callerAssetsHelper {
		required = append(required,
			`[[ -f "$assets_helper" && ! -L "$assets_helper" ]]`,
			`source "$assets_helper"`, "verify_public_key_lineage_assets")
	}
	if p.callerReleaseID {
		required = append(required, `[[ "$(jq -er '.id' "$release_json")" == "$prior_release_id" ]]`)
	}
	return required
}
