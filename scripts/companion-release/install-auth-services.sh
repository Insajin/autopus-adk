#!/usr/bin/env bash
# Install the OMP auth broker and gateway as resident launchd user agents.
#
# The release procedure needs a loopback provider gateway. Starting it by hand
# before each attempt is a step that gets forgotten, and the failure surfaces
# deep inside prep rather than at the start. Residency removes the step.
#
# Two deliberate choices:
#
#   The services run the *installed* omp, not the release-pinned binary. The
#   gateway is a credential proxy; the evidence oracle is a separate invocation
#   that prep makes with its own pinned digest. Coupling a resident service to
#   the release pin would mean reinstalling the service every time the pin moves.
#
#   Both bind loopback only and keep their bearer auth. The broker is a
#   credential vault holding subscription OAuth, so it is reachable from this
#   machine and nowhere else, and a caller still needs the token.
set -euo pipefail
umask 077

fail() { printf 'install auth services: %s\n' "$1" >&2; exit 1; }
usage() {
  printf '%s\n' 'usage: install-auth-services.sh [--broker-port N] [--gateway-port N] [--uninstall]' >&2
  exit 64
}

broker_port=47311
gateway_port=47312
uninstall=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --broker-port) [[ $# -ge 2 ]] || usage; broker_port=$2; shift 2 ;;
    --gateway-port) [[ $# -ge 2 ]] || usage; gateway_port=$2; shift 2 ;;
    --uninstall) uninstall=1; shift ;;
    *) usage ;;
  esac
done
for port in "$broker_port" "$gateway_port"; do
  [[ "$port" =~ ^[1-9][0-9]{3,4}$ ]] || fail "port $port is not a plausible high port"
done
[[ "$broker_port" != "$gateway_port" ]] || fail 'broker and gateway ports must differ'

[[ "$(uname -s)" == 'Darwin' ]] || fail 'launchd residency is macOS only'

readonly agents="$HOME/Library/LaunchAgents"
readonly logs="$HOME/Library/Logs/autopus"
readonly broker_label='co.autopus.omp-auth-broker'
readonly gateway_label='co.autopus.omp-auth-gateway'

if [[ "$uninstall" -eq 1 ]]; then
  for label in "$gateway_label" "$broker_label"; do
    launchctl bootout "gui/$(id -u)/${label}" 2>/dev/null || true
    rm -f "$agents/${label}.plist"
    printf '  removed %s\n' "$label"
  done
  exit 0
fi

omp=$(command -v omp) || fail 'omp is not on PATH'
[[ -x "$omp" ]] || fail "omp at $omp is not executable"
"$omp" auth-gateway --help >/dev/null 2>&1 || fail 'installed omp has no auth-gateway subcommand'

mkdir -p "$agents" "$logs"

write_agent() {
  local label=$1 plist="$agents/$1.plist"
  shift
  # Program arguments arrive as remaining positional parameters; the caller also
  # sets broker_env when the agent needs to reach the broker.
  {
    printf '%s\n' '<?xml version="1.0" encoding="UTF-8"?>'
    printf '%s\n' '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">'
    printf '%s\n' '<plist version="1.0"><dict>'
    printf '  <key>Label</key><string>%s</string>\n' "$label"
    printf '%s\n' '  <key>ProgramArguments</key><array>'
    for argument in "$@"; do printf '    <string>%s</string>\n' "$argument"; done
    printf '%s\n' '  </array>'
    printf '%s\n' '  <key>RunAtLoad</key><true/>'
    # KeepAlive rather than ordering: launchd has no dependency graph, so the
    # gateway simply retries until the broker answers.
    printf '%s\n' '  <key>KeepAlive</key><true/>'
    printf '%s\n' '  <key>ThrottleInterval</key><integer>5</integer>'
    printf '  <key>StandardOutPath</key><string>%s/%s.log</string>\n' "$logs" "$label"
    printf '  <key>StandardErrorPath</key><string>%s/%s.err.log</string>\n' "$logs" "$label"
    printf '%s\n' '  <key>EnvironmentVariables</key><dict>'
    printf '    <key>PATH</key><string>%s</string>\n' '/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin'
    printf '    <key>HOME</key><string>%s</string>\n' "$HOME"
    if [[ -n "${broker_env:-}" ]]; then
      printf '    <key>OMP_AUTH_BROKER_URL</key><string>%s</string>\n' "$broker_env"
    fi
    printf '%s\n' '  </dict>'
    printf '%s\n' '</dict></plist>'
  } >"$plist"
  chmod 0600 "$plist"
  launchctl bootout "gui/$(id -u)/${label}" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$plist" || fail "cannot bootstrap $label"
  printf '  installed %s\n' "$label"
}

broker_env='' write_agent "$broker_label" \
  "$omp" auth-broker serve --bind "127.0.0.1:${broker_port}"

broker_env="http://127.0.0.1:${broker_port}" write_agent "$gateway_label" \
  "$omp" auth-gateway serve --bind "127.0.0.1:${gateway_port}"

printf '\nwaiting for the gateway to report ready\n'
ready=0
for _ in 1 2 3 4 5 6 7 8 9 10; do
  if OMP_AUTH_BROKER_URL="http://127.0.0.1:${broker_port}" \
    "$omp" auth-gateway status --json 2>/dev/null | grep -q '"ready":true'; then
    ready=1
    break
  fi
  sleep 2
done
[[ "$ready" -eq 1 ]] || fail "gateway did not report ready; see $logs/${gateway_label}.err.log"

cat <<EOF

gateway ready on http://127.0.0.1:${gateway_port}

Add these to your shell profile so release-prep.sh finds them after a reboot:

  export OMP_AUTH_BROKER_URL=http://127.0.0.1:${broker_port}
  export ADK_GATEWAY_URL=http://127.0.0.1:${gateway_port}

Remove the services with: $0 --uninstall
EOF
