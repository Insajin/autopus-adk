#!/usr/bin/env bash
set -euo pipefail

fail() { printf 'release tag ruleset: %s\n' "$1" >&2; exit 1; }
readonly repository='Insajin/autopus-adk'
readonly ruleset_name='autopus-v0.50.109-release-authority'
readonly release_ref='refs/tags/v0.50.109'
readonly release_actor_id=204883817
readonly environment_name='adk-companion-release'
for tool in gh jq; do command -v "$tool" >/dev/null || fail "${tool} is unavailable"; done

summaries=$(gh api "repos/${repository}/rulesets?includes_parents=true&targets=tag") ||
  fail 'cannot inspect release tag rulesets'
ruleset_id=$(jq -er --arg name "$ruleset_name" '
  [.[] | select(.name == $name and .target == "tag")] |
  if length == 1 then .[0].id else error("ruleset is missing or ambiguous") end
' <<<"$summaries") || fail 'release tag ruleset is missing or ambiguous'
[[ "$ruleset_id" =~ ^[1-9][0-9]*$ ]] || fail 'release tag ruleset ID is malformed'
ruleset=$(gh api "repos/${repository}/rulesets/${ruleset_id}") ||
  fail 'cannot read exact release tag ruleset'
jq -e --arg name "$ruleset_name" --arg ref "$release_ref" \
  --argjson actor "$release_actor_id" '
  .name == $name and .target == "tag" and .enforcement == "active" and
  .conditions.ref_name.include == [$ref] and .conditions.ref_name.exclude == [] and
  .bypass_actors == [{actor_id:$actor,actor_type:"User",bypass_mode:"always"}] and
  ([.rules[].type] | sort) == ["creation","deletion","update"] and
  ([.rules[].type] | unique | length) == 3
' <<<"$ruleset" >/dev/null || fail 'release tag ruleset differs from exact authority policy'
environment=$(gh api "repos/${repository}/environments/${environment_name}") ||
  fail 'cannot inspect protected release environment'
jq -e --argjson actor "$release_actor_id" '
  .can_admins_bypass == false and
  .deployment_branch_policy == {
    protected_branches:false,
    custom_branch_policies:true
  } and
  ([.protection_rules[] | select(.type == "required_reviewers")] | length) == 1 and
  ([.protection_rules[] | select(.type == "required_reviewers")][0] |
    .prevent_self_review == false and
    [.reviewers[] | {type:.type,id:.reviewer.id}] == [{type:"User",id:$actor}]) and
  ([.protection_rules[] | select(.type == "branch_policy")] | length) == 1
' <<<"$environment" >/dev/null ||
  fail 'protected release environment differs from exact authority policy'
