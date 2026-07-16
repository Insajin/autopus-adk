#!/usr/bin/env bash
# Test-only GitHub Contents API state machine.
set -euo pipefail

[[ "${1-}" == 'api' ]]
shift
method='GET'
input=''
endpoint=''
while (($#)); do
  case "$1" in
    --method) method="$2"; shift 2 ;;
    --input) input="$2"; shift 2 ;;
    -H) shift 2 ;;
    *) endpoint="$1"; shift ;;
  esac
done

case "$endpoint" in
  *Casks/auto.rb*) name='cask' ;;
  *Formula/auto.rb*) name='formula' ;;
  *) exit 64 ;;
esac
state="$MOCK_TAP_STATE/${name}.json"

if [[ "$method" == 'GET' ]]; then
  exec cat "$state"
fi
[[ "$method" == 'PUT' && -f "$input" ]]
current_sha=$(jq -er '.sha' "$state")
[[ "$(jq -er '.sha' "$input")" == "$current_sha" ]]
[[ "$(jq -er '.branch' "$input")" == 'main' ]]
content=$(jq -er '.content' "$input")
if [[ "$name" == 'cask' ]]; then
  new_sha='1111111111111111111111111111111111111111'
else
  new_sha='2222222222222222222222222222222222222222'
fi
jq -n --arg sha "$new_sha" --arg content "$content" \
  '{sha:$sha,content:$content}' >"$state"
count_file="$MOCK_TAP_STATE/${name}.updates"
count=$(cat "$count_file" 2>/dev/null || printf '0')
printf '%s\n' "$((count + 1))" >"$count_file"
printf '{}\n'
