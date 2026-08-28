#!/usr/bin/env bash
set -euo pipefail

fail() { printf 'rotation ref ruleset: %s\n' "$1" >&2; exit 1; }
readonly repository='Insajin/autopus-adk'
readonly ruleset_name='autopus-v0.50.109-rotation-ref-authority'
readonly rotation_ref='refs/heads/release-key-rotation-v0.50.109'
readonly publisher_actor_id=204883817
for tool in gh jq; do command -v "$tool" >/dev/null || fail "${tool} is unavailable"; done

summaries=$(gh api "repos/${repository}/rulesets?includes_parents=true&targets=branch") ||
  fail 'cannot inspect rotation ref rulesets'
ruleset_id=$(jq -er --arg name "$ruleset_name" '
  [.[] | select(.name == $name and .target == "branch")] |
  if length == 1 then .[0].id else error("ruleset is missing or ambiguous") end
' <<<"$summaries") || fail 'rotation ref ruleset is missing or ambiguous'
[[ "$ruleset_id" =~ ^[1-9][0-9]*$ ]] || fail 'rotation ref ruleset ID is malformed'
ruleset=$(gh api "repos/${repository}/rulesets/${ruleset_id}") ||
  fail 'cannot read exact rotation ref ruleset'
jq -e --arg name "$ruleset_name" --arg ref "$rotation_ref" \
  --argjson actor "$publisher_actor_id" '
  .name == $name and .target == "branch" and .enforcement == "active" and
  .conditions.ref_name.include == [$ref] and .conditions.ref_name.exclude == [] and
  .bypass_actors == [{actor_id:$actor,actor_type:"User",bypass_mode:"always"}] and
  ([.rules[].type] | sort) == ["creation","deletion","update"] and
  ([.rules[].type] | unique | length) == 3
' <<<"$ruleset" >/dev/null || fail 'rotation ref ruleset differs from exact authority policy'
