#!/bin/sh
set -eu

usage() {
  echo "usage: verify-changed-go-coverage.sh --base <sha> --profile <coverprofile> --min <percent> --baseline <json>" >&2
  exit 2
}

base='' profile='' minimum='' baseline=''
while [ "$#" -gt 0 ]; do
  case "$1" in
    --base) [ "$#" -ge 2 ] || usage; base=$2; shift 2 ;;
    --profile) [ "$#" -ge 2 ] || usage; profile=$2; shift 2 ;;
    --min) [ "$#" -ge 2 ] || usage; minimum=$2; shift 2 ;;
    --baseline) [ "$#" -ge 2 ] || usage; baseline=$2; shift 2 ;;
    *) usage ;;
  esac
done
[ -n "$base" ] && [ -n "$profile" ] && [ -n "$minimum" ] && [ -n "$baseline" ] || usage
git cat-file -e "$base^{commit}" 2>/dev/null || { echo "coverage gate: invalid base" >&2; exit 1; }
[ -f "$profile" ] || { echo "coverage gate: profile missing" >&2; exit 1; }
[ -f "$baseline" ] || { echo "coverage gate: baseline missing" >&2; exit 1; }
case "$minimum" in *[!0-9.]*|''|.*|*.*.*|*.) echo "coverage gate: invalid minimum" >&2; exit 1;; esac
awk -v value="$minimum" 'BEGIN { exit !(value >= 0 && value <= 100) }' || {
  echo "coverage gate: invalid minimum" >&2; exit 1;
}

work=$(mktemp -d "${TMPDIR:-/tmp}/autopus-coverage.XXXXXX")
trap 'rm -rf "$work"' EXIT HUP INT TERM
{
  git diff --name-only --diff-filter=ACMR "$base" -- '*.go'
  git ls-files --others --exclude-standard -- '*.go'
} | sort -u > "$work/changed"

# Auditable SPEC-OMP-003 production ownership contract. Do not broaden this to omp_*.go.
awk '
  /_test\.go$/ { next }
  $0 == "pkg/config/schema.go" { print; next }
  $0 ~ /^pkg\/config\/role_model_policy.*\.go$/ { print; next }
  $0 == "pkg/content/agent_transformer_omp.go" { print; next }
  $0 == "pkg/adapter/omp/omp_agents.go" { print; next }
  $0 ~ /^internal\/cli\/doctor_omp_model_routing.*\.go$/ { print; next }
  $0 ~ /^pkg\/adapter\/omp\/omp_model_(activation|catalog|doctor|integration|projection|receipt|routing)(_[a-z0-9_]+)?\.go$/ { print; next }
' "$work/changed" > "$work/selected"
[ -s "$work/selected" ] || { echo "coverage gate: no routing-owned production files" >&2; exit 1; }

awk -v selected="$work/selected" -v package_out="$work/packages" '
  BEGIN { while ((getline line < selected) > 0) wanted[line]=1; close(selected) }
  NR == 1 { if ($0 !~ /^mode: /) exit 20; next }
  {
    if ($2 !~ /^[0-9]+$/ || $3 !~ /^[0-9]+$/) exit 23
    split($1, location, ":"); path=location[1]
    if (match(path, /\/pkg\//)) path=substr(path, RSTART+1)
    else if (match(path, /\/internal\//)) path=substr(path, RSTART+1)
    statements=$2+0; count=$3+0
    slash=path; sub(/\/[^\/]+$/, "", slash)
    package_total[slash]+=statements; if (count>0) package_covered[slash]+=statements
    if (wanted[path]) { seen[path]=1; total+=statements; if (count>0) covered+=statements }
  }
  END {
    if (NR < 2) exit 21
    for (path in wanted) if (wanted[path] && !seen[path]) { print "missing:" path > "/dev/stderr"; missing=1 }
    if (missing || total == 0) exit 22
    printf "%.6f\n", 100*covered/total
    for (pkg in package_total) printf "%s %.6f\n", pkg, 100*package_covered[pkg]/package_total[pkg] > package_out
  }
' "$profile" > "$work/aggregate" || { echo "coverage gate: malformed profile or missing row" >&2; exit 1; }

aggregate=$(cat "$work/aggregate")
awk -v got="$aggregate" -v want="$minimum" 'BEGIN { exit !(got+0 >= want+0) }' || {
  echo "coverage gate: aggregate below minimum" >&2; exit 1;
}

cat > "$work/baseline_parser.go" <<'EOF'
package main
import (
  "encoding/json"
  "fmt"
  "io"
  "os"
  "sort"
  "strings"
)
func main() {
  file, err := os.Open(os.Args[1]); if err != nil { os.Exit(1) }
  defer file.Close()
  var value struct { Packages map[string]float64 `json:"packages"` }
  decoder := json.NewDecoder(file); decoder.DisallowUnknownFields()
  if decoder.Decode(&value) != nil || len(value.Packages) == 0 { os.Exit(1) }
  var trailing any; if decoder.Decode(&trailing) != io.EOF { os.Exit(1) }
  keys := make([]string, 0, len(value.Packages))
  for key, percent := range value.Packages {
    if (!strings.HasPrefix(key, "pkg/") && !strings.HasPrefix(key, "internal/")) || percent < 0 || percent > 100 { os.Exit(1) }
    keys = append(keys, key)
  }
  sort.Strings(keys)
  for _, key := range keys { fmt.Printf("%s %.6f\n", key, value.Packages[key]) }
}
EOF
go run "$work/baseline_parser.go" "$baseline" > "$work/baseline-packages" 2>/dev/null || {
  echo "coverage gate: malformed baseline" >&2; exit 1;
}

while read -r package expected; do
  current=$(awk -v package="$package" '$1 == package { print $2 }' "$work/packages")
  [ -n "$current" ] || { echo "coverage gate: baseline package missing from profile" >&2; exit 1; }
  awk -v got="$current" -v want="$expected" 'BEGIN { exit !(got+0.000001 >= want+0) }' || {
    echo "coverage gate: package baseline regression" >&2; exit 1;
  }
done < "$work/baseline-packages"

echo "coverage gate: pass aggregate=${aggregate}% files=$(wc -l < "$work/selected" | tr -d ' ')"
