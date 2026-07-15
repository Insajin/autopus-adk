#!/usr/bin/env bash

emit_diag_full5() {
  local seq=$1 arm=$2 ordinal
  for ordinal in 1 2 3; do
    seq=$((seq + 1))
    printf '%s\tute-corpus-v1-006\t%s\tAB\tfull5\treviewer\t%s\txhigh\t22000\n' "$seq" "$arm" "$ordinal"
  done
  seq=$((seq + 1))
  printf '%s\tute-corpus-v1-006\t%s\tAB\tfull5\tsecurity-auditor\t1\tmax\t26000\n' "$seq" "$arm"
  seq=$((seq + 1))
  printf '%s\tute-corpus-v1-006\t%s\tAB\tfull5\treview-consolidator\t1\txhigh\t22000\n' "$seq" "$arm"
  DIAG_SCHEDULE_SEQ=$seq
}

emit_diag_schedule() {
  emit_diag_full5 0 A
  emit_diag_full5 "$DIAG_SCHEDULE_SEQ" B
}

validate_diag_schedule() {
  local file=$1 calls xhigh max raw
  calls=$(wc -l < "$file" | tr -d ' ')
  xhigh=$(awk -F '\t' '$8 == "xhigh" {n++} END {print n+0}' "$file")
  max=$(awk -F '\t' '$8 == "max" {n++} END {print n+0}' "$file")
  raw=$(awk -F '\t' '{n += $9} END {print n+0}' "$file")
  [[ "$calls" == 10 && "$xhigh" == 8 && "$max" == 2 && "$raw" == 228000 ]]
}
