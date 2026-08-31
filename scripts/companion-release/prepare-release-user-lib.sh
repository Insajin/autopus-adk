#!/usr/bin/env bash
# Ephemeral account metadata for numeric-UID canary identity resolution.

release_canary_account_value() {
  local attribute=$1
  /usr/bin/dscl . -read "/Users/${release_canary_user}" "$attribute" 2>/dev/null |
    /usr/bin/awk '
      NR == 1 { sub(/^([^:]+:)+[[:space:]]*/, ""); value = $0; next }
      {
        gsub(/^[[:space:]]+|[[:space:]]+$/, "")
        value = value (value == "" ? "" : " ") $0
      }
      END {
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
        print value
      }
    '
}

verify_release_canary_account() {
  [[ "$release_canary_user" =~ ^_autopus_v110_[0-9a-f]{8}$ &&
     "$release_canary_uid" == '59999' && "$release_canary_gid" == '20' &&
     "$release_canary_home" =~ ^/private/tmp/autopus-adk-release-prep-[0-9a-f]{32}-final/home$ &&
     "$release_canary_marker" =~ ^Autopus\ Release\ Canary\ [0-9a-f]{32}$ ]] || return 1
  [[ "$(release_canary_account_value UniqueID)" == "$release_canary_uid" &&
     "$(release_canary_account_value PrimaryGroupID)" == "$release_canary_gid" &&
     "$(release_canary_account_value NFSHomeDirectory)" == "$release_canary_home" &&
     "$(release_canary_account_value UserShell)" == '/usr/bin/false' &&
     "$(release_canary_account_value RealName)" == "$release_canary_marker" &&
     "$(release_canary_account_value IsHidden)" == '1' ]]
}

clear_release_canary_account_state() {
  release_canary_user=''
  release_canary_uid=''
  release_canary_gid=''
  release_canary_home=''
  release_canary_marker=''
  release_canary_account_created=0
}

create_release_canary_account() {
  local home=$1 account_name account_uid
  [[ -z "${release_canary_user:-}" && -z "${release_canary_uid:-}" &&
     "$home" =~ ^/private/tmp/autopus-adk-release-prep-[0-9a-f]{32}-final/home$ ]] ||
    fail 'dedicated live-canary account state is invalid'
  release_canary_user="_autopus_v110_${dispatch_nonce:0:8}"
  release_canary_uid='59999'
  release_canary_gid='20'
  release_canary_home=$home
  release_canary_marker="Autopus Release Canary ${dispatch_nonce}"
  if /usr/bin/dscl . -read "/Users/${release_canary_user}" >/dev/null 2>&1; then
    fail 'dedicated live-canary account already exists'
  fi
  while read -r account_name account_uid; do
    [[ "$account_uid" != "$release_canary_uid" ]] ||
      fail 'dedicated live-canary UID is already assigned'
  done < <(/usr/bin/dscl . -list /Users UniqueID)
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" ||
    fail 'create dedicated live-canary account'
  release_canary_account_created=1
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" RealName "$release_canary_marker" ||
    fail 'mark dedicated live-canary account'
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" UniqueID "$release_canary_uid" ||
    fail 'set dedicated live-canary UID'
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" PrimaryGroupID "$release_canary_gid" ||
    fail 'set dedicated live-canary group'
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" NFSHomeDirectory "$release_canary_home" ||
    fail 'set dedicated live-canary home'
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" UserShell /usr/bin/false ||
    fail 'disable dedicated live-canary login shell'
  /usr/bin/sudo -n /usr/bin/dscl . -create "/Users/${release_canary_user}" IsHidden 1 ||
    fail 'hide dedicated live-canary account'
  verify_release_canary_account || fail 'dedicated live-canary account differs after creation'
  /usr/bin/pgrep -u "$release_canary_uid" >/dev/null 2>&1 &&
    fail 'dedicated live-canary account started an unexpected process'
}

remove_release_canary_account() {
  [[ -n "${release_canary_user:-}" ]] || return 0
  if [[ "${release_canary_account_created:-0}" -ne 1 ]]; then return 1; fi
  /usr/bin/pgrep -u "$release_canary_uid" >/dev/null 2>&1 && return 1
  if ! verify_release_canary_account; then
    /usr/bin/sudo -n /usr/bin/dscl . -delete "/Users/${release_canary_user}" || return 1
    /usr/bin/dscl . -read "/Users/${release_canary_user}" >/dev/null 2>&1 && return 1
    clear_release_canary_account_state
    return 0
  fi
  /usr/bin/sudo -n /usr/bin/dscl . -delete "/Users/${release_canary_user}" || return 1
  /usr/bin/dscl . -read "/Users/${release_canary_user}" >/dev/null 2>&1 && return 1
  clear_release_canary_account_state
}
