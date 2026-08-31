package run

import "strings"

// captureProducerScript renders a fake Playwright producer.
//
// It exercises the real handoff rather than stubbing the oracle: the script only
// sees the exported environment, writes files into the harness-allocated capture
// directory, and computes true digests and byte counts. `mode` selects a specific
// contract violation so each failure path is tested through the actual runner.
func captureProducerScript(mode string) string {
	script := captureProducerBase
	switch mode {
	case "skip-index":
		script = strings.Replace(script, "WRITE_INDEX=1", "WRITE_INDEX=0", 1)
	case "bad-totals":
		script = strings.Replace(script, `"console_errors": 0`, `"console_errors": 41`, 1)
	case "no-receipt":
		script = strings.Replace(script, "WRITE_RECEIPT=1", "WRITE_RECEIPT=0", 1)
	case "off-origin":
		script = strings.Replace(script, `"url_ref": "/api/cart"`, `"url_ref": "origin:7/api/cart"`, 1)
	case "drop-trace":
		script = strings.Replace(script, "DROP_TRACE=0", "DROP_TRACE=1", 1)
	}
	return script
}

const captureProducerBase = `#!/bin/sh
set -e
WRITE_INDEX=1
WRITE_RECEIPT=1
DROP_TRACE=0
mkdir -p .autopus/qa/gui
[ -f "$AUTOPUS_QAMESH_GUI_POLICY_PATH" ] || exit 7
[ -d "$AUTOPUS_QAMESH_GUI_CAPTURE_DIR" ] || exit 10
[ -f "$AUTOPUS_QAMESH_GUI_CAPTURE_DIR/gui-capture-policy.json" ] || exit 11
[ -n "$AUTOPUS_QAMESH_GUI_CAPTURE_INDEX_PATH" ] || exit 12

# The real guard preload writes this receipt from inside the Playwright workers.
# It is the only witness the harness trusts for policy enforcement, so the fake
# producer must emit it too or the runtime oracle correctly reports it missing.
if [ "$WRITE_RECEIPT" = "1" ] && [ -n "$AUTOPUS_QAMESH_GUI_GUARD_RECEIPT_PATH" ]; then
  mkdir -p "$(dirname "$AUTOPUS_QAMESH_GUI_GUARD_RECEIPT_PATH")"
  printf '{"t":"goto","origin":"http://127.0.0.1:4173","allowed":true}\n' >> "$AUTOPUS_QAMESH_GUI_GUARD_RECEIPT_PATH"
fi

CAP="$AUTOPUS_QAMESH_GUI_CAPTURE_DIR"
mkdir -p "$CAP/screenshots" "$CAP/traces"
printf 'fake-png-bytes' > "$CAP/screenshots/open.png"
printf 'fake-trace-bytes' > "$CAP/traces/journey.zip"

digest() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    sha256sum "$1" | cut -d' ' -f1
  fi
}

SHOT_D=$(digest "$CAP/screenshots/open.png")
SHOT_B=$(wc -c < "$CAP/screenshots/open.png" | tr -d ' ')
TRACE_D=$(digest "$CAP/traces/journey.zip")
TRACE_B=$(wc -c < "$CAP/traces/journey.zip" | tr -d ' ')
MEDIA_B=$((SHOT_B + TRACE_B))

if [ "$WRITE_INDEX" = "1" ]; then
cat > "$AUTOPUS_QAMESH_GUI_CAPTURE_INDEX_PATH" <<EOF
{
  "schema_version": "qamesh.gui_capture_index.v1",
  "journey_id": "gui-capture-smoke",
  "mode": "always",
  "streams": ["screenshot", "console", "network", "trace"],
  "started_at": "2026-01-02T03:00:00Z",
  "ended_at": "2026-01-02T03:00:05Z",
  "steps": [
    {
      "step_id": "open-home",
      "order": 1,
      "title": "open home",
      "status": "passed",
      "started_at": "2026-01-02T03:00:00Z",
      "ended_at": "2026-01-02T03:00:05Z",
      "duration_ms": 5000,
      "actions": [{"api": "goto", "target_ref": "origin:0/", "duration_ms": 120}],
      "screenshot": {
        "ref": "screenshots/open.png",
        "digest": "sha256:$SHOT_D",
        "bytes": $SHOT_B,
        "width": 1280,
        "height": 720,
        "retention": "local_only"
      },
      "console": {"errors": 0, "warnings": 1, "messages": [{"severity": "warning", "text": "slow hydration"}]},
      "network": {"requests": 2, "failures": 0, "entries": [{"method": "GET", "url_ref": "/api/cart", "status": 200, "duration_ms": 30}]}
    }
  ],
  "media": [
    {
      "kind": "trace",
      "step_id": "open-home",
      "ref": "traces/journey.zip",
      "digest": "sha256:$TRACE_D",
      "bytes": $TRACE_B,
      "retention": "local_only"
    }
  ],
  "totals": {
    "steps": 1,
    "actions": 1,
    "console_errors": 0,
    "network_failures": 0,
    "screenshots": 1,
    "media_bytes": $MEDIA_B
  }
}
EOF
fi

if [ "$DROP_TRACE" = "1" ]; then
  rm -f "$CAP/traces/journey.zip"
fi

echo gui capture ok
exit 0
`
