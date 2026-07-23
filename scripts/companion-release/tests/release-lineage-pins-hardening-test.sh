#!/usr/bin/env bash
set -euo pipefail
umask 077

tests_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
script_dir=$(cd -- "$tests_dir/.." && pwd)
fail() { printf 'release lineage pins hardening test: %s\n' "$1" >&2; exit 1; }
contains() { grep -Fq -- "$2" "$1" || fail "$1 missing $2"; }
not_contains() { ! grep -Fq -- "$2" "$1" || fail "$1 unexpectedly contains $2"; }

source_gate="$script_dir/validate-source.sh"
lineage="$script_dir/verify-public-key-lineage.sh"
lineage_pins="$script_dir/verify-public-key-lineage-pins.sh"
lineage_coordinates="$script_dir/verify-public-key-lineage-coordinates.sh"
lineage_assets="$script_dir/verify-public-key-lineage-assets.sh"

while IFS= read -r declaration; do
  contains "$source_gate" "$declaration"
done <<'ANCESTORS'
readonly A6_A5_ANCESTOR_SHA='b27252cb1148192a8ae1a95195c50e5f221453a4'
readonly A7_A6_ANCESTOR_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'
readonly A8_A7_ANCESTOR_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'
readonly A9_A8_ANCESTOR_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'
readonly A10_A9_ANCESTOR_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'
readonly A11_A10_ANCESTOR_SHA='54536edc09c37a634532c2c9b51e62869d393db4'
readonly A12_A11_ANCESTOR_SHA='a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9'
readonly A13_A12_ANCESTOR_SHA='e6367b5375cd4cdf09cb1515877bc57323521364'
readonly A14_A13_ANCESTOR_SHA='2b7aa046bdb7861113dfa57b30489c11715582e9'
readonly A15_A14_ANCESTOR_SHA='4b8eb62200d253b46e022670c482e2f716a992a3'
ANCESTORS

for helper_name in pins coordinates archive assets; do
  contains "$lineage" "[[ -f \"\$${helper_name}_helper\" && ! -L \"\$${helper_name}_helper\" ]]"
  contains "$lineage" "source \"\$${helper_name}_helper\""
done
contains "$lineage_assets" 'verify_public_key_lineage_assets()'
contains "$lineage_assets" '_linux_amd64.tar.gz'
contains "$lineage_assets" '_linux_arm64.tar.gz'
[[ "$(grep -Fc 'extract_bundle ' "$lineage_assets")" == '2' ]] \
  || fail 'lineage asset helper must extract only two Darwin bundles'

while IFS= read -r declaration; do
  contains "$lineage_pins" "$declaration"
done <<'PINS'
A4_TAG_OBJECT_SHA='b1ebab0af82536f8a4bc1ed93f31f82f6c53d008'
A4_CHECKSUMS_SHA256='a30e0893f1565919e9e90dd7e1f2b19e5487024b0373f66de56729e1d747e7d1'
A4_AMD64_ARCHIVE_SHA256='da7f6ef4396591ff0b728f976536d261ecb084038fffab7c7662a6f7329ade2a'
A4_ARM64_ARCHIVE_SHA256='ff046f6af316236166d514608a1b432c2f3a01efbd8aab03b54d2c2639d2f422'
A4_AMD64_MANIFEST_SHA256='86940b9c7eb89308aff4260d9a6178d933d3f1a9833e601ac8c1e914c225a7b5'
A4_ARM64_MANIFEST_SHA256='a68a10a46b0778ccc858855323fd45cf0b9727f76fa45b16efdbc83b320128f0'
A5_TAG_OBJECT_SHA='c79f133f0108bf3f07cee0162c1abeecf9d379d1'
A5_CHECKSUMS_SHA256='48c79e1fb47444aa83909794cd041bdfed18bf263bf5c0209578540382824ad4'
A5_AMD64_ARCHIVE_SHA256='aeb9d048579c77ab17f4a4ec3a1160778d16c627747c5af5f341e664e1417cb0'
A5_ARM64_ARCHIVE_SHA256='bc90e594c91de61dabc2982f60249b638d448fa3f6643004fe6d45cdd0cc5eab'
A5_AMD64_MANIFEST_SHA256='5b4381d3f2180b19c0da9d419ebc8452b9ba04c73c8d0921c2a74c09ab38b85c'
A5_ARM64_MANIFEST_SHA256='62a9f78302ee000c16c1c73669282e955fc3abc82f850ff4a77d0e04069f4aed'
A6_COMMIT_SHA='902f1acfa91f1d0a2ac9471d5cd79117031a2599'
A6_TAG_OBJECT_SHA='41feed7decafac33d8f7f43e06804e3c9bf37ef3'
A6_CHECKSUMS_SHA256='fb1a35dcdb44255aad43b7ae74950ed59f05ccf44abde9cadf28ecfa0dfce37a'
A6_AMD64_ARCHIVE_SHA256='d5e47076c1fc898d2b3f5880b6edfcf9a12e805633dcba2691da22f300d41dc9'
A6_ARM64_ARCHIVE_SHA256='d6d092177a5406c194eea1de4fbd11b8af92a03814eb143a294541a3a578b9ab'
A6_AMD64_MANIFEST_SHA256='64c634130b16a74cbb33f666d316a05d9a7a1012246dc58fde6e15350b71d0c5'
A6_ARM64_MANIFEST_SHA256='b6611c04990b048bc5545e37c942bc8e7e4fab8592d546eaab80d7084991bea6'
A7_COMMIT_SHA='51de6030a69a8e36fcf7e5790ef157eff6fedf00'
A7_TREE_SHA='3cd00b17bd8bd6aa8def213de1c5765c3611765d'
A7_TAG_OBJECT_SHA='417a318fb6a11a720e2c4102e92e39ea9ed676e9'
A7_CHECKSUMS_SHA256='322d2ef21dff55f02ca36944aba88ee5da92fdae6bcd16a89319f1697efb9733'
A7_AMD64_ARCHIVE_SHA256='43018046ab37027b7fba3888d288961cb5abc136e478deaa9f878586bcce6629'
A7_ARM64_ARCHIVE_SHA256='e72653fd3094537caa60398e2017d409796d7ceef88a7662ca93b6299e9d00ec'
A7_AMD64_MANIFEST_SHA256='3f7c879c93dea0d119805987bef434b65c1a53684e80f78b5d9a0c9c2cd011d5'
A7_ARM64_MANIFEST_SHA256='87ef2a30d6ee8c9abe9e679d597d0a4fbe9bb5cdee1266572476ad6a66aef975'
A8_COMMIT_SHA='dd0c2759ed5435d4634011e349caad62ea3df414'
A8_TREE_SHA='4325913ba332c583dd573ccf9248b38497d76926'
A8_TAG_OBJECT_SHA='8c6dcef91407e3321704014559cfd929d14768d0'
A8_CHECKSUMS_SHA256='1d0bdbfe50f85c381fde11c334c97a1b783dcfa4e12e0c4023152f38119a0bcd'
A8_AMD64_ARCHIVE_SHA256='19e317cdabc9dde976ca772d9ddbbf693b444dd44eefa70c8d0313a32de89a9b'
A8_ARM64_ARCHIVE_SHA256='41e29ae1c3c48dd6e3e5f4dfe8076472704d00a7d479b5cc8a90f53c0af6ef31'
A8_AMD64_MANIFEST_SHA256='c5ac37874bac5de87152e781bd82a17c7705894f24be81657ccc907f15ba1f65'
A8_ARM64_MANIFEST_SHA256='ebcf563c11f0836be2b2bd4423ea315283eeec12cfa200d479e1a56f5909f5f1'
A9_COMMIT_SHA='c9c4f49d48022eb0c8d72ee7b520136a4f21f176'
A9_TREE_SHA='3a71fa56bd917f447a6b1705772b6ab99bbcfbc8'
A9_TAG_OBJECT_SHA='b7d05fa76eed41b1dfb4eddbd9873525e0aac15f'
A9_CHECKSUMS_SHA256='9ed1f99d22a761abb7953c70aab3c7de5ab0b7ec3524cf3798fcd3815c53bde7'
A9_AMD64_ARCHIVE_SHA256='48f80577ff2ef40a843dab0a847895ca7b3877e7fb810a30d328cbe8a55fc51e'
A9_ARM64_ARCHIVE_SHA256='503c338e1ce122e209b9e74bc883492317144b319b0713943bc299e57447024d'
A9_AMD64_MANIFEST_SHA256='589f02503aa02338ed14d67b1eb6b31e2b96a9e83b47c99e5cd5a31b75ede9b7'
A9_ARM64_MANIFEST_SHA256='ffdd6ccbecff2b8ea38bc5c5f65ff7f078b229bd4658f90d08bb5e801c184a7f'
A10_COMMIT_SHA='54536edc09c37a634532c2c9b51e62869d393db4'
A10_TREE_SHA='e9a30f4530e06c9b62933e7bf97e0056faed259c'
A10_TAG_OBJECT_SHA='8b37fccb57255fc24003dc3af2700334f4a8d3c4'
A10_CHECKSUMS_SHA256='2e97c1f3c8d0cba0f93dd83c724c71eaa4966c79d4812a6a9cf034144c7b178d'
A10_AMD64_ARCHIVE_SHA256='b745eaddd8c70cb415aca42901213ffeb3c1d567f9b889e87a4a895ecfda8134'
A10_ARM64_ARCHIVE_SHA256='71a40ee709f34fb29bb562cde4587e2da1db1d6e8bc300d0edb4cfe63f8bec3c'
A10_AMD64_MANIFEST_SHA256='98b38d8d59c5d146234e5a5f9bae26e80f8af0f699ac23e3f9fed5e59b32321e'
A10_ARM64_MANIFEST_SHA256='976aa2bbeedd4e32b522373f6bf75a93b15f6813c4373c638c27d2cb98e4f00a'
A11_COMMIT_SHA='a8558ccc36e04125de6b8d84c7ffc9e8ddb5a2c9'
A11_TREE_SHA='9545ed7437e6dfd7573952586a31964061e30e2d'
A11_TAG_OBJECT_SHA='c636f42a6e8dc65ef6500eb95dac4ef7d1faff9a'
A11_CHECKSUMS_SHA256='a7973f9fa27d1e0ca1d1943adcfe5be0fa6807ba0517ff9066b2659fa6f4f01c'
A11_AMD64_ARCHIVE_SHA256='f5825b4aff8ce84e6b18dfb0ae0249a432a1b247477c3a9e2cd14689a405d40d'
A11_ARM64_ARCHIVE_SHA256='c913c51b396e01034e889f43ef4da68fcae851e7f7cba7f2b8ac60a2c4e00c66'
A11_AMD64_MANIFEST_SHA256='5a036574b0cfe8fa62dfe3dde3d65d248ed225aa883c898caced3d55906b47ba'
A11_ARM64_MANIFEST_SHA256='990b9f1cfb0768db4bb23719006320d845b72322fa9fddc2317ab75381b734ee'
A12_COMMIT_SHA='e6367b5375cd4cdf09cb1515877bc57323521364'
A12_TREE_SHA='6c9a22e85d5a8c5f23c0d9e1bb41de270cab85a4'
A12_TAG_OBJECT_SHA='080507fceb3b4bf31f0e0887e49013fd65645ac2'
A12_CHECKSUMS_SHA256='7d871b077766f3a7dd6859427fa9b1333422312764820243d3bf7af5e935dee0'
A12_AMD64_ARCHIVE_SHA256='da92acfa4e8f45a0abea90b0991ae87cc7fb345c4f1ca2c166a8626670df658b'
A12_ARM64_ARCHIVE_SHA256='5b29fdb21b62f8933c1ff0608f9c1dca096be24649fd24ec40bcbe9ff72c4fcc'
A12_AMD64_MANIFEST_SHA256='caa1145bc293a125495795914005429694e2a2b98a863d903a40575495ec250a'
A12_ARM64_MANIFEST_SHA256='013e7b98bfea64783d932e787609d526d5157801788b90b13cc59990070ab20b'
A13_COMMIT_SHA='2b7aa046bdb7861113dfa57b30489c11715582e9'
A13_TREE_SHA='95d1b00bcc1cb1bfcca3dd58e1e5e1b94575c367'
A13_TAG_OBJECT_SHA='de34e9c1a2a06b27f57235c81a59d1da180eab6d'
A13_CHECKSUMS_SHA256='8f00d3b42d71c9e71346bf62cd72f8e1428600cb0795f703d90de64b3b9ba14e'
A13_AMD64_ARCHIVE_SHA256='fa60e03ecd39a5fa203be3cca3e8a7010e3af7854195f0e866ef80e7a0e82f0f'
A13_ARM64_ARCHIVE_SHA256='f4ed0ef8d6f0274389ada5cebdeb87a2899bf34b7a11bd99318b5914775d84f1'
A13_AMD64_MANIFEST_SHA256='ba6f3e92d4a1c0a1a52b7b17e484961cb8640944eae24856652ebe6192210931'
A13_ARM64_MANIFEST_SHA256='22660fc029bbcb9ffe312964d9f674ba2587440dba48790e28fb4f35b19dcc69'
A14_COMMIT_SHA='4b8eb62200d253b46e022670c482e2f716a992a3'
A14_TREE_SHA='fbdc83287982899c3d6bfe5fdf7b88494e76bcb0'
A14_TAG_OBJECT_SHA='f005dd935dbbcec8c60052adcfda6632fe8831e1'
A14_CHECKSUMS_SHA256='5bd11e327eab31c555f89298761e2d27bca2fadebfc3b7961cafb6a140539236'
A14_AMD64_ARCHIVE_SHA256='66834d509309cb09b84f78bb81a97e68a8d03434c9a37f239a2ae04677dbc10b'
A14_ARM64_ARCHIVE_SHA256='7fe10bc7b03b3df44f803622e3830e5e91f3ea12b47b706cf14f716b076b012e'
A14_LINUX_AMD64_ARCHIVE_SHA256='187620011ce035f6bdb09f3f6d5b005f878463c3ba0fd805142cbd3e4f587698'
A14_LINUX_ARM64_ARCHIVE_SHA256='654e42612a3f1ee670157cd461b3dff1270f2102b085984951975c0284356172'
A14_AMD64_MANIFEST_SHA256='4265d3f18c7aaab779a720216c2f1dfc9a486c01be898290d4f56be31102008e'
A14_ARM64_MANIFEST_SHA256='918c91d4bdee0c58e74e0068314d35463e094fef214986a550579bca08b2ef38'
PINS

for phase in A8 A9 A10 A11 A12 A13 A14 A15; do
  prior=$((10#${phase#A} - 1))
  contains "$lineage_coordinates" "release_phase='${phase}' prior_phase='A${prior}'"
done
contains "$lineage_coordinates" \
  "prior_tree='' prior_linux_amd64_archive='' prior_linux_arm64_archive=''"
for phase in {0..13}; do
  not_contains "$lineage_pins" "A${phase}_LINUX_"
done

temp=$(mktemp -d "${TMPDIR:-/tmp}/release-lineage-helper-gate.XXXXXX")
cleanup() { local status=$?; rm -rf -- "$temp" || status=$?; return "$status"; }
trap cleanup EXIT
for helper in pins coordinates archive; do
  install -m 0600 "$script_dir/verify-public-key-lineage-${helper}.sh" \
    "$temp/verify-public-key-lineage-${helper}.sh"
done
install -m 0700 "$lineage" "$temp/verify-public-key-lineage.sh"
ln -s -- "$lineage_assets" "$temp/verify-public-key-lineage-assets.sh"
if output=$(GITHUB_REF_NAME=v0.50.86 PATH="$PATH" \
  bash "$temp/verify-public-key-lineage.sh" 2>&1); then
  fail 'symlinked lineage asset helper passed'
fi
[[ "$output" == *'lineage asset verifier is invalid'* ]] \
  || fail "symlinked lineage asset helper diagnostic = ${output}"

printf 'release lineage pins hardening test: PASS\n'
