#!/usr/bin/env bash
set -euo pipefail

state=${MOCK_RELEASE_PREP_STATE:?}
log="$state/calls.log"
command=${1-}; shift || true
scope_file() {
  local environment=''
  for ((index=1; index<=$#; index++)); do
    if [[ "${!index}" == '--env' ]]; then
      index=$((index + 1)); environment=${!index}
    fi
  done
  if [[ -n "$environment" ]]; then printf '%s\n' "$state/environment-variables.json"
  else printf '%s\n' "$state/repository-variables.json"; fi
}
write_count() {
  local count
  count=$(<"$state/write-count")
  count=$((count + 1)); printf '%s\n' "$count" >"$state/write-count"
  if [[ -n "${MOCK_RELEASE_PREP_FAIL_AT:-}" && "$count" == "$MOCK_RELEASE_PREP_FAIL_AT" ]]; then
    exit 75
  fi
  if [[ -n "${MOCK_RELEASE_PREP_FAIL_FROM:-}" &&
    "$count" -ge "$MOCK_RELEASE_PREP_FAIL_FROM" ]]; then
    exit 75
  fi
}
case "$command" in
  variable)
    action=${1-}; name=${2-}; shift 2 || true
    file=$(scope_file "$@")
    case "$action" in
      list) cat "$file" ;;
      get)
        jq -er --arg name "$name" '.[] | select(.name == $name) | .value' "$file" ;;
      set)
        body=''
        while [[ $# -gt 0 ]]; do
          case "$1" in --body) body=$2; shift 2 ;; *) shift ;; esac
        done
        write_count
        jq --arg name "$name" --arg value "$body" \
          '[.[] | select(.name != $name)] + [{name:$name,value:$value}] | sort_by(.name)' \
          "$file" >"$file.next"
        mv "$file.next" "$file"
        printf 'variable-set\t%s\t%s\n' "$(basename "$file")" "$name" >>"$log"
        ;;
      delete)
        write_count
        jq --arg name "$name" '[.[] | select(.name != $name)]' "$file" >"$file.next"
        mv "$file.next" "$file"
        printf 'variable-delete\t%s\t%s\n' "$(basename "$file")" "$name" >>"$log"
        ;;
      *) exit 64 ;;
    esac
    ;;
  api)
    method=GET endpoint='' field_name='' field_type='' field_tag='' field_target='' field_release_name='' field_body='' field_draft='' field_prerelease=''
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --method) method=$2; shift 2 ;;
        -f|-F|--field|--raw-field)
          case "$2" in
            name=*) field_name=${2#name=}; field_release_name=${2#name=} ;;
            type=*) field_type=${2#type=} ;;
            tag_name=*) field_tag=${2#tag_name=} ;;
            target_commitish=*) field_target=${2#target_commitish=} ;;
            body=*) field_body=${2#body=} ;;
            draft=*) field_draft=${2#draft=} ;;
            prerelease=*) field_prerelease=${2#prerelease=} ;;
          esac
          shift 2
          ;;
        --jq) shift 2 ;;
        --silent) shift ;;
        -*) shift ;;
        *) [[ -z "$endpoint" ]] && endpoint=$1; shift ;;
      esac
    done
    case "$method:$endpoint" in
      GET:repos/Insajin/autopus-adk) printf '%s\n' '{"permissions":{"admin":true}}' ;;
      'GET:repos/Insajin/autopus-adk/releases?per_page=100')
        visibility_delay=${MOCK_RELEASE_PREP_RELEASE_VISIBILITY_DELAY:-0}
        if [[ -f "$state/release-created.json" && "$visibility_delay" -gt 0 ]]; then
          visibility_delay_file="$state/release-visibility-delay"
          if [[ ! -f "$visibility_delay_file" ]]; then
            printf '%s\n' "$visibility_delay" >"$visibility_delay_file"
          fi
          remaining_visibility_delay=$(<"$visibility_delay_file")
          if [[ "$remaining_visibility_delay" -gt 0 ]]; then
            printf '%s\n' "$((remaining_visibility_delay - 1))" >"$visibility_delay_file"
            printf '%s\n' 'release-reservation-not-yet-visible' >>"$log"
            jq -c '[[.[] | select(.id != 996)]]' "$state/releases.json"
            exit 0
          fi
        fi
        jq -c '[.]' "$state/releases.json"
        ;;
      GET:repos/Insajin/autopus-adk/releases/tags/v0.50.104)
        jq -ce '.[] | select(.tag_name == "v0.50.104")' "$state/releases.json"
        ;;
      POST:repos/Insajin/autopus-adk/releases)
        [[ "$field_tag" == 'v0.50.104' && "$field_target" =~ ^[0-9a-f]{40}$ &&
           "$field_release_name" == 'v0.50.104' &&
           "$field_draft" == 'true' && "$field_prerelease" == 'false' ]] || exit 65
        jq -e --arg tag "$field_tag" 'all(.[]; .tag_name != $tag)' \
          "$state/releases.json" >/dev/null || exit 65
        write_count
        jq -n --arg tag "$field_tag" --arg target "$field_target" \
          --arg name "$field_release_name" --arg body "$field_body" \
          '{id:996,tag_name:$tag,target_commitish:$target,name:$name,body:$body,draft:true,prerelease:false,author:{id:204883817},assets:[]}' \
          >"$state/release-created.json"
        jq -s '.[0] + [.[1]]' "$state/releases.json" "$state/release-created.json" \
          >"$state/releases.json.next"
        mv "$state/releases.json.next" "$state/releases.json"
        printf '%s\n' 'release-reservation-create' >>"$log"
        if [[ "${MOCK_RELEASE_PREP_RELEASE_RESPONSE_LOST:-0}" -eq 1 ]]; then exit 75; fi
        cat "$state/release-created.json"
        ;;
      DELETE:repos/Insajin/autopus-adk/releases/996)
        if [[ "${MOCK_RELEASE_PREP_RELEASE_DELETE_FAIL:-0}" -eq 1 ]]; then exit 75; fi
        write_count
        jq '[.[] | select(.id != 996)]' "$state/releases.json" >"$state/releases.json.next"
        mv "$state/releases.json.next" "$state/releases.json"
        printf '%s\n' 'release-reservation-delete' >>"$log"
        ;;
      GET:repos/Insajin/autopus-adk/environments/adk-companion-release)
        printf '%s\n' '{"deployment_branch_policy":{"custom_branch_policies":true,"protected_branches":false}}'
        ;;
      GET:repos/Insajin/autopus-adk/environments/adk-companion-release/deployment-branch-policies)
        jq -n --slurpfile policies "$state/deployment-policies.json" '{branch_policies:$policies[0]}'
        ;;
      POST:repos/Insajin/autopus-adk/environments/adk-companion-release/deployment-branch-policies)
        [[ "$field_name" == 'v0.50.104' && "$field_type" == 'tag' ]] || exit 65
        write_count
        jq '[.[] | select(.name != "v0.50.104")] + [{id:596,type:"tag",name:"v0.50.104"}]' \
          "$state/deployment-policies.json" >"$state/deployment-policies.json.next"
        mv "$state/deployment-policies.json.next" "$state/deployment-policies.json"
        printf '%s\n' 'policy-create' >>"$log"
        if [[ "${MOCK_RELEASE_PREP_POLICY_RESPONSE_LOST:-0}" -eq 1 ]]; then
          exit 75
        fi
        printf '%s\n' '{"id":596,"type":"tag","name":"v0.50.104"}'
        ;;
      DELETE:repos/Insajin/autopus-adk/environments/adk-companion-release/deployment-branch-policies/596)
        write_count
        jq '[.[] | select(.id != 596)]' "$state/deployment-policies.json" >"$state/deployment-policies.json.next"
        mv "$state/deployment-policies.json.next" "$state/deployment-policies.json"
        printf '%s\n' 'policy-delete' >>"$log"
        ;;
      *) exit 64 ;;
    esac
    ;;
  *) exit 64 ;;
esac
