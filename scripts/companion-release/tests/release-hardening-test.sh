#!/usr/bin/env bash
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
repo=$(cd -- "$script_dir/../.." && pwd)
fail() { printf 'release hardening test: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$1 missing $2"; }
not_contains() { ! grep -Fq -- "$2" "$1" || fail "$1 unexpectedly contains $2"; }

config="$repo/.goreleaser.yaml"
release="$repo/.github/workflows/release.yaml"
recovery="$repo/.github/workflows/homebrew-formula-bridge-recovery.yaml"
source_gate="$script_dir/validate-source.sh"
lineage="$script_dir/verify-public-key-lineage.sh"
lineage_coordinates="$script_dir/verify-public-key-lineage-coordinates.sh"
lineage_pins="$script_dir/verify-public-key-lineage-pins.sh"
producer="$script_dir/produce.sh"
producer_receipt="$script_dir/produce-public-key-receipt.sh"
homebrew_bridge="$script_dir/publish-homebrew-formula-bridge.sh"
homebrew_git_helper="$script_dir/publish-homebrew-formula-bridge-git.sh"
current_release_gate="$script_dir/verify-current-release.sh"
current_signature_gate="$script_dir/verify-current-release-signatures.sh"

# GoReleaser must render, but never publish, the Cask or mutate tagged source.
contains "$config" 'skip_upload: true'
not_contains "$config" 'token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"'
not_contains "$config" 'go mod tidy'
contains "$release" 'go mod tidy -diff'

# A fresh, narrowly scoped tap token must be created after immutable release publish.
release_index=$(grep -n 'goreleaser release --clean' "$release" | cut -d: -f1)
signing_cleanup_index=$(grep -n 'name: Remove release signing credentials' "$release" | cut -d: -f1)
evidence_index=$(grep -n 'scripts/companion-release/verify-current-release.sh' "$release" | cut -d: -f1)
token_index=$(grep -n 'name: Create Homebrew tap token' "$release" | cut -d: -f1)
bridge_index=$(grep -n 'scripts/companion-release/publish-homebrew-formula-bridge.sh' "$release" | cut -d: -f1)
(( release_index < signing_cleanup_index && signing_cleanup_index < evidence_index && \
   evidence_index < token_index && token_index < bridge_index )) \
  || fail 'release evidence or tap token ordering is unsafe'
goreleaser_step=$(sed -n '/name: Run GoReleaser/,/name: Verify current immutable release evidence/p' "$release")
[[ "$goreleaser_step" != *HOMEBREW_TAP_TOKEN* ]] || fail 'GoReleaser receives tap token'
[[ "$goreleaser_step" != *APPLE_CERTIFICATE_PASSWORD* ]] || fail 'GoReleaser receives certificate password'
contains "$release" "COMPANION_CASK_PATH='dist/homebrew/Casks/auto.rb'"
contains "$release" 'COMPANION_CHECKSUMS_PATH: ${{ steps.release-evidence.outputs.checksums-path }}'
contains "$release" 'COMPANION_CHECKSUMS_PATH="$COMPANION_CHECKSUMS_PATH"'
not_contains "$release" "COMPANION_CHECKSUMS_PATH='dist/checksums.txt'"
contains "$producer_receipt" '--signing-key "$COMPANION_SIGNING_KEY_FILE"'
contains "$homebrew_bridge" "readonly PRIOR_TAP_COMMIT='192cacd10d0c85d5cc0533356400e697152a551c'"
contains "$homebrew_bridge" "readonly PRIOR_CASK_BLOB='2ba9ab9caa381c68a276588a7d6ad77de46f1dd5'"
contains "$homebrew_bridge" "readonly FROZEN_FORMULA_BLOB='4ebc6c38925002dec00759823d4dd847a499818a'"
contains "$homebrew_bridge" 'COMPANION_HOMEBREW_POLICY'
contains "$homebrew_bridge" "readonly FORMULA_PATH='Formula/auto.rb'"
contains "$homebrew_bridge" 'source "$git_helper"'
contains "$homebrew_git_helper" 'verify_frozen_formula'
not_contains "$homebrew_git_helper" 'reconcile_tap_file formula Formula'
contains "$homebrew_git_helper" "api_json POST 'git/blobs'"
contains "$homebrew_git_helper" "api_json POST 'git/trees'"
contains "$homebrew_git_helper" "api_json POST 'git/commits'"
contains "$homebrew_git_helper" 'api_json PATCH "git/refs/heads/${TAP_BRANCH}"'
contains "$homebrew_git_helper" '{base_tree:$base,tree:['
contains "$homebrew_git_helper" "'{sha:\$sha,force:false}'"
not_contains "$homebrew_git_helper" '--method PUT'

# Production/recovery source coordinates must bind to externally approved exact values.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_COMMIT'
  contains "$workflow" 'ADK_COMPANION_APPROVED_SOURCE_TREE'
  contains "$workflow" 'COMPANION_SOURCE_PIN_REQUIRED='
done
contains "$release" "- 'v0.50.84'"
contains "$release" "if: github.ref == 'refs/tags/v0.50.84'"
contains "$recovery" "if: github.ref == 'refs/tags/v0.50.84'"
contains "$recovery" 'gh workflow run homebrew-formula-bridge-recovery.yaml --ref v0.50.84'
not_contains "$release" "'v0.50.83'"
not_contains "$release" 'refs/tags/v0.50.83'
not_contains "$recovery" 'refs/tags/v0.50.83'
not_contains "$release" "'v0.50.82'"
not_contains "$release" 'refs/tags/v0.50.82'
not_contains "$recovery" 'refs/tags/v0.50.82'
not_contains "$release" "'v0.50.81'"
not_contains "$release" 'refs/tags/v0.50.81'
not_contains "$recovery" 'refs/tags/v0.50.81'
not_contains "$release" "'v0.50.79'"
not_contains "$release" 'refs/tags/v0.50.79'
not_contains "$recovery" 'refs/tags/v0.50.79'
not_contains "$release" "'v0.50.77'"
not_contains "$release" 'refs/tags/v0.50.77'
not_contains "$recovery" 'refs/tags/v0.50.77'
not_contains "$release" "'v0.50.75'"
not_contains "$release" 'refs/tags/v0.50.75'
not_contains "$recovery" 'refs/tags/v0.50.75'
not_contains "$release" "'v0.50.76'"
not_contains "$release" 'refs/tags/v0.50.76'
not_contains "$recovery" 'refs/tags/v0.50.76'
contains "$source_gate" "readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'"
contains "$source_gate" "readonly A7_A6_ANCESTOR_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'"
contains "$source_gate" "readonly A8_A7_ANCESTOR_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'"
contains "$source_gate" "readonly A9_A8_ANCESTOR_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'"
contains "$source_gate" "readonly A10_A9_ANCESTOR_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'"
contains "$source_gate" "readonly A11_A10_ANCESTOR_SHA='54536edc09c37a634532c2c9b51e62869d393db4'"
contains "$source_gate" "readonly A12_A11_ANCESTOR_SHA='a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9'"
contains "$source_gate" "readonly A13_A12_ANCESTOR_SHA='e6367b5375cd4cdf09cb1515877bc57323521364'"
contains "$lineage" 'source "$pins_helper"'
contains "$lineage" 'source "$coordinates_helper"'
contains "$lineage_pins" "A4_TAG_OBJECT_SHA='b1ebab0af82536f8a4bc1ed93f31f82f6c53d008'"
contains "$lineage_pins" "A4_CHECKSUMS_SHA256='a30e0893f1565919e9e90dd7e1f2b19e5487024b0373f66de56729e1d747e7d1'"
contains "$lineage_pins" "A4_AMD64_ARCHIVE_SHA256='da7f6ef4396591ff0b728f976536d261ecb084038fffab7c7662a6f7329ade2a'"
contains "$lineage_pins" "A4_ARM64_ARCHIVE_SHA256='ff046f6af316236166d514608a1b432c2f3a01efbd8aab03b54d2c2639d2f422'"
contains "$lineage_pins" "A4_AMD64_MANIFEST_SHA256='86940b9c7eb89308aff4260d9a6178d933d3f1a9833e601ac8c1e914c225a7b5'"
contains "$lineage_pins" "A4_ARM64_MANIFEST_SHA256='a68a10a46b0778ccc858855323fd45cf0b9727f76fa45b16efdbc83b320128f0'"
contains "$lineage_pins" "A5_TAG_OBJECT_SHA='c79f133f0108bf3f07cee0162c1abeecf9d379d1'"
contains "$lineage_pins" "A5_CHECKSUMS_SHA256='48c79e1fb47444aa83909794cd041bdfed18bf263bf5c0209578540382824ad4'"
contains "$lineage_pins" "A5_AMD64_ARCHIVE_SHA256='aeb9d048579c77ab17f4a4ec3a1160778d16c627747c5af5f341e664e1417cb0'"
contains "$lineage_pins" "A5_ARM64_ARCHIVE_SHA256='bc90e594c91de61dabc2982f60249b638d448fa3f6643004fe6d45cdd0cc5eab'"
contains "$lineage_pins" "A5_AMD64_MANIFEST_SHA256='5b4381d3f2180b19c0da9d419ebc8452b9ba04c73c8d0921c2a74c09ab38b85c'"
contains "$lineage_pins" "A5_ARM64_MANIFEST_SHA256='62a9f78302ee000c16c1c73669282e955fc3abc82f850ff4a77d0e04069f4aed'"
contains "$lineage_pins" "A6_COMMIT_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'"
contains "$lineage_pins" "A6_TAG_OBJECT_SHA='41feed7decafac33d8f7f43e06804e3c9bf37ef3'"
contains "$lineage_pins" "A6_CHECKSUMS_SHA256='fb1a35dcdb44255aad43b7ae74950ed59f05ccf44abde9cadf28ecfa0dfce37a'"
contains "$lineage_pins" "A6_AMD64_ARCHIVE_SHA256='d5e47076c1fc898d2b3f5880b6edfcf9a12e805633dcba2691da22f300d41dc9'"
contains "$lineage_pins" "A6_ARM64_ARCHIVE_SHA256='d6d092177a5406c194eea1de4fbd11b8af92a03814eb143a294541a3a578b9ab'"
contains "$lineage_pins" "A6_AMD64_MANIFEST_SHA256='64c634130b16a74cbb33f666d316a05d9a7a1012246dc58fde6e15350b71d0c5'"
contains "$lineage_pins" "A6_ARM64_MANIFEST_SHA256='b6611c04990b048bc5545e37c942bc8e7e4fab8592d546eaab80d7084991bea6'"
contains "$lineage_pins" "A7_COMMIT_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'"
contains "$lineage_pins" "A7_TREE_SHA='3cd00b17bd8bd6aa8def213de1c5765c3611765d'"
contains "$lineage_pins" "A7_TAG_OBJECT_SHA='417a318fb6a11a720e2c4102e92e39ea9ed676e9'"
contains "$lineage_pins" "A7_CHECKSUMS_SHA256='322d2ef21dff55f02ca36944aba88ee5da92fdae6bcd16a89319f1697efb9733'"
contains "$lineage_pins" "A7_AMD64_ARCHIVE_SHA256='43018046ab37027b7fba3888d288961cb5abc136e478deaa9f878586bcce6629'"
contains "$lineage_pins" "A7_ARM64_ARCHIVE_SHA256='e72653fd3094537caa60398e2017d409796d7ceef88a7662ca93b6299e9d00ec'"
contains "$lineage_pins" "A7_AMD64_MANIFEST_SHA256='3f7c879c93dea0d119805987bef434b65c1a53684e80f78b5d9a0c9c2cd011d5'"
contains "$lineage_pins" "A7_ARM64_MANIFEST_SHA256='87ef2a30d6ee8c9abe9e679d597d0a4fbe9bb5cdee1266572476ad6a66aef975'"
contains "$lineage_pins" "A8_COMMIT_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'"
contains "$lineage_pins" "A8_TREE_SHA='4325913ba332c583dd573ccf9248b38497d76926'"
contains "$lineage_pins" "A8_TAG_OBJECT_SHA='8c6dcef91407e3321704014559cfd929d14768d0'"
contains "$lineage_pins" "A8_CHECKSUMS_SHA256='1d0bdbfe50f85c381fde11c334c97a1b783dcfa4e12e0c4023152f38119a0bcd'"
contains "$lineage_pins" "A8_AMD64_ARCHIVE_SHA256='19e317cdabc9dde976ca772d9ddbbf693b444dd44eefa70c8d0313a32de89a9b'"
contains "$lineage_pins" "A8_ARM64_ARCHIVE_SHA256='41e29ae1c3c48dd6e3e5f4dfe8076472704d00a7d479b5cc8a90f53c0af6ef31'"
contains "$lineage_pins" "A8_AMD64_MANIFEST_SHA256='c5ac37874bac5de87152e781bd82a17c7705894f24be81657ccc907f15ba1f65'"
contains "$lineage_pins" "A8_ARM64_MANIFEST_SHA256='ebcf563c11f0836be2b2bd4423ea315283eeec12cfa200d479e1a56f5909f5f1'"
contains "$lineage_pins" "A9_COMMIT_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'"
contains "$lineage_pins" "A9_TREE_SHA='3a71fa56bd917f447a6b1705772b6ab99bbcfbc8'"
contains "$lineage_pins" "A9_TAG_OBJECT_SHA='b7d05fa76eed41b1dfb4eddbd9873525e0aac15f'"
contains "$lineage_pins" "A9_CHECKSUMS_SHA256='9ed1f99d22a761abb7953c70aab3c7de5ab0b7ec3524cf3798fcd3815c53bde7'"
contains "$lineage_pins" "A9_AMD64_ARCHIVE_SHA256='48f80577ff2ef40a843dab0a847895ca7b3877e7fb810a30d328cbe8a55fc51e'"
contains "$lineage_pins" "A9_ARM64_ARCHIVE_SHA256='503c338e1ce122e209b9e74bc883492317144b319b0713943bc299e57447024d'"
contains "$lineage_pins" "A9_AMD64_MANIFEST_SHA256='589f02503aa02338ed14d67b1eb6b31e2b96a9e83b47c99e5cd5a31b75ede9b7'"
contains "$lineage_pins" "A9_ARM64_MANIFEST_SHA256='ffdd6ccbecff2b8ea38bc5c5f65ff7f078b229bd4658f90d08bb5e801c184a7f'"
contains "$lineage_pins" "A10_COMMIT_SHA='54536edc09c37a634532c2c9b51e62869d393db4'"
contains "$lineage_pins" "A10_TREE_SHA='e9a30f4530e06c9b62933e7bf97e0056faed259c'"
contains "$lineage_pins" "A10_TAG_OBJECT_SHA='8b37fccb57255fc24003dc3af2700334f4a8d3c4'"
contains "$lineage_pins" "A10_CHECKSUMS_SHA256='2e97c1f3c8d0cba0f93dd83c724c71eaa4966c79d4812a6a9cf034144c7b178d'"
contains "$lineage_pins" "A10_AMD64_ARCHIVE_SHA256='b745eaddd8c70cb415aca42901213ffeb3c1d567f9b889e87a4a895ecfda8134'"
contains "$lineage_pins" "A10_ARM64_ARCHIVE_SHA256='71a40ee709f34fb29bb562cde4587e2da1db1d6e8bc300d0edb4cfe63f8bec3c'"
contains "$lineage_pins" "A10_AMD64_MANIFEST_SHA256='98b38d8d59c5d146234e5a5f9bae26e80f8af0f699ac23e3f9fed5e59b32321e'"
contains "$lineage_pins" "A10_ARM64_MANIFEST_SHA256='976aa2bbeedd4e32b522373f6bf75a93b15f6813c4373c638c27d2cb98e4f00a'"
contains "$lineage_pins" "A11_COMMIT_SHA='a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9'"
contains "$lineage_pins" "A11_TREE_SHA='9545ed7437e6dfd7573952586a31964061e30e2d'"
contains "$lineage_pins" "A11_TAG_OBJECT_SHA='c636f42a6e8dc65ef6500eb95dac4ef7d1faff9a'"
contains "$lineage_pins" "A11_CHECKSUMS_SHA256='a7973f9fa27d1e0ca1d1943adcfe5be0fa6807ba0517ff9066b2659fa6f4f01c'"
contains "$lineage_pins" "A11_AMD64_ARCHIVE_SHA256='f5825b4aff8ce84e6b18dfb0ae0249a432a1b247477c3a9e2cd14689a405d40d'"
contains "$lineage_pins" "A11_ARM64_ARCHIVE_SHA256='c913c51b396e01034e889f43ef4da68fcae851e7f7cba7f2b8ac60a2c4e00c66'"
contains "$lineage_pins" "A11_AMD64_MANIFEST_SHA256='5a036574b0cfe8fa62dfe3dde3d65d248ed225aa883c898caced3d55906b47ba'"
contains "$lineage_pins" "A11_ARM64_MANIFEST_SHA256='990b9f1cfb0768db4bb23719006320d845b72322fa9fddc2317ab75381b734ee'"
contains "$lineage_pins" "A12_COMMIT_SHA='e6367b5375cd4cdf09cb1515877bc57323521364'"
contains "$lineage_pins" "A12_TREE_SHA='6c9a22e85d5a8c5f23c0d9e1bb41de270cab85a4'"
contains "$lineage_pins" "A12_TAG_OBJECT_SHA='080507fceb3b4bf31f0e0887e49013fd65645ac2'"
contains "$lineage_pins" "A12_CHECKSUMS_SHA256='7d871b077766f3a7dd6859427fa9b1333422312764820243d3bf7af5e935dee0'"
contains "$lineage_pins" "A12_AMD64_ARCHIVE_SHA256='da92acfa4e8f45a0abea90b0991ae87cc7fb345c4f1ca2c166a8626670df658b'"
contains "$lineage_pins" "A12_ARM64_ARCHIVE_SHA256='5b29fdb21b62f8933c1ff0608f9c1dca096be24649fd24ec40bcbe9ff72c4fcc'"
contains "$lineage_pins" "A12_AMD64_MANIFEST_SHA256='caa1145bc293a125495795914005429694e2a2b98a863d903a40575495ec250a'"
contains "$lineage_pins" "A12_ARM64_MANIFEST_SHA256='013e7b98bfea64783d932e787609d526d5157801788b90b13cc59990070ab20b'"
contains "$lineage_coordinates" "release_phase='A8' prior_phase='A7'"
contains "$lineage_coordinates" "release_phase='A9' prior_phase='A8'"
contains "$lineage_coordinates" "release_phase='A10' prior_phase='A9'"
contains "$lineage_coordinates" "release_phase='A11' prior_phase='A10'"
contains "$lineage_coordinates" "release_phase='A12' prior_phase='A11'"
contains "$lineage_coordinates" "release_phase='A13' prior_phase='A12'"
contains "$lineage" '.commit.tree.sha'
contains "$producer_receipt" "GITHUB_REF_NAME\" == 'v0.50.84'"
contains "$producer_receipt" "release_phase='A13'"
contains "$homebrew_bridge" "readonly RELEASE_TAG='v0.50.84'"
contains "$homebrew_bridge" "readonly RELEASE_VERSION='0.50.84'"
contains "$release" 'timeout-minutes: 60'
contains "$recovery" 'timeout-minutes: 20'

# Production and recovery must share one exact, fail-closed current-release gate.
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'scripts/companion-release/verify-current-release.sh'
  workflow_evidence_index=$(grep -n 'scripts/companion-release/verify-current-release.sh' "$workflow" | cut -d: -f1)
  workflow_token_index=$(grep -n 'name: Create Homebrew tap token' "$workflow" | cut -d: -f1)
  (( workflow_evidence_index < workflow_token_index )) || fail 'tap token precedes release evidence'
done
contains "$current_release_gate" "readonly RELEASE_TAG='v0.50.84'"
contains "$current_release_gate" "readonly RELEASE_VERSION='0.50.84'"
contains "$current_release_gate" '.target_commitish == $commit'
contains "$current_release_gate" '.immutable == true'
contains "$current_release_gate" '(.assets | length) == ($expected | length)'
contains "$current_release_gate" '[.assets[].name] | unique | length'
contains "$current_release_gate" '.state == "uploaded"'
contains "$current_release_gate" '.size > 0'
contains "$current_release_gate" '^sha256:[0-9a-f]{64}$'
contains "$current_release_gate" 'for archive in "${EXPECTED_ARCHIVES[@]}"'
contains "$current_release_gate" 'verify-current-release-signatures.sh'
contains "$current_release_gate" 'env -i PATH="$PATH" HOME="${HOME:-/}" TMPDIR="${TMPDIR:-/tmp}"'
contains "$current_release_gate" "download_release_asset 'checksums.txt.bundle'"
contains "$current_release_gate" "download_release_asset 'checksums.txt.signatures'"
contains "$current_signature_gate" 'verify_release_checksums_v1'
contains "$current_signature_gate" 'cosign verify-blob'
contains "$current_signature_gate" 'unset GITHUB_TOKEN GH_TOKEN'
for workflow in "$release" "$recovery"; do
  contains "$workflow" 'sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6'
  contains "$workflow" "cosign-release: 'v3.1.2'"
  not_contains "$workflow" 'sigstore/cosign-installer@59acb6260d9c0ba8f4a2f9d9b48431a222b68e20'
done

bash "$tests_dir/release-runtime-hardening-test.sh"
bash "$tests_dir/release-exec-smoke-hardening-test.sh"
bash "$tests_dir/release-homebrew-hardening-test.sh"
bash "$tests_dir/release-producer-helper-hardening-test.sh"
bash "$tests_dir/release-current-signature-hardening-test.sh"

printf 'release hardening test: PASS\n'
