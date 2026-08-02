#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
gate="$script_dir/verify-changed-go-coverage.sh"
fixture=$(mktemp -d "${TMPDIR:-/tmp}/autopus-coverage-test.XXXXXX")
trap 'rm -rf "$fixture"' EXIT HUP INT TERM
cd "$fixture"
git init -q
git config user.email fixture@example.invalid
git config user.name fixture
mkdir -p pkg/adapter/omp
printf 'package omp\n' > pkg/adapter/omp/base.go
git add . && git commit -qm base
base=$(git rev-parse HEAD)
printf 'package omp\nfunc route() {}\n' > pkg/adapter/omp/omp_model_routing.go
cat > profile.out <<'EOF'
mode: set
example/pkg/adapter/omp/base.go:1.12,1.12 0 1
example/pkg/adapter/omp/omp_model_routing.go:2.14,2.16 2 1
example/pkg/adapter/omp/omp_model_routing.go:2.17,2.18 1 0
example/pkg/adapter/omp/base.go:1.1,1.12 1 1
EOF
printf '{"packages":{"pkg/adapter/omp":70}}\n' > baseline.json
"$gate" --base "$base" --profile profile.out --min 60 --baseline baseline.json >/dev/null
if "$gate" --base "$base" --profile profile.out --min 90 --baseline baseline.json >/dev/null 2>&1; then exit 1; fi
sed '/omp_model_routing/d' profile.out > missing.out
if "$gate" --base "$base" --profile missing.out --min 0 --baseline baseline.json >/dev/null 2>&1; then exit 1; fi
printf '{"packages":{"pkg/adapter/omp":90}}\n' > regression.json
if "$gate" --base "$base" --profile profile.out --min 0 --baseline regression.json >/dev/null 2>&1; then exit 1; fi
printf '{malformed\n' > malformed.json
if "$gate" --base "$base" --profile profile.out --min 0 --baseline malformed.json >/dev/null 2>&1; then exit 1; fi
echo "coverage gate self-test: pass"
