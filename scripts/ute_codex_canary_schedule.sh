#!/usr/bin/env bash

emit_full_profile() {
  local seq=$1 task=$2 arm=$3 order=$4
  local ordinal
  for ordinal in 1 2 3; do
    seq=$((seq + 1))
    printf '%s\t%s\t%s\t%s\tfull5\treviewer\t%s\txhigh\t22000\n' \
      "$seq" "$task" "$arm" "$order" "$ordinal"
  done
  seq=$((seq + 1))
  printf '%s\t%s\t%s\t%s\tfull5\tsecurity-auditor\t1\tmax\t26000\n' \
    "$seq" "$task" "$arm" "$order"
  seq=$((seq + 1))
  printf '%s\t%s\t%s\t%s\tfull5\treview-consolidator\t1\txhigh\t22000\n' \
    "$seq" "$task" "$arm" "$order"
  SCHEDULE_SEQ=$seq
}

emit_compact_profile() {
  local seq=$1 task=$2 arm=$3 order=$4
  seq=$((seq + 1))
  printf '%s\t%s\t%s\t%s\tcompact2\treviewer\t1\txhigh\t22000\n' \
    "$seq" "$task" "$arm" "$order"
  seq=$((seq + 1))
  printf '%s\t%s\t%s\t%s\tcompact2\tsecurity-auditor\t1\tmax\t26000\n' \
    "$seq" "$task" "$arm" "$order"
  SCHEDULE_SEQ=$seq
}

candidate_profile_for_task() {
  case "$1" in
    *-001|*-004|*-011|*-012) printf '%s\n' compact2 ;;
    *-005|*-006|*-009) printf '%s\n' full5 ;;
    *) return 1 ;;
  esac
}

emit_primary_schedule() {
  local seq=0 task order arm profile
  while IFS=$'\t' read -r task order; do
    for arm in "${order:0:1}" "${order:1:1}"; do
      profile=full5
      [[ "$arm" == B ]] && profile=$(candidate_profile_for_task "$task")
      if [[ "$profile" == full5 ]]; then
        emit_full_profile "$seq" "$task" "$arm" "$order"
      else
        emit_compact_profile "$seq" "$task" "$arm" "$order"
      fi
      seq=$SCHEDULE_SEQ
    done
  done <<'EOF'
ute-corpus-v1-001	AB
ute-corpus-v1-004	BA
ute-corpus-v1-005	AB
ute-corpus-v1-011	BA
ute-corpus-v1-012	AB
ute-corpus-v1-006	BA
ute-corpus-v1-009	AB
EOF
}

emit_rollback_schedule() {
  emit_full_profile 0 ute-corpus-v1-001 R NA
}

validate_schedule_arithmetic() {
  local schedule=$1 calls xhigh max raw
  calls=$(wc -l < "$schedule" | tr -d ' ')
  xhigh=$(awk -F '\t' '$8 == "xhigh" {n++} END {print n+0}' "$schedule")
  max=$(awk -F '\t' '$8 == "max" {n++} END {print n+0}' "$schedule")
  raw=$(awk -F '\t' '{n += $9} END {print n+0}' "$schedule")
  if [[ "$calls" == 58 ]]; then
    [[ "$xhigh" == 44 && "$max" == 14 && "$raw" == 1332000 ]] || return 1
  elif [[ "$calls" == 5 ]]; then
    [[ "$xhigh" == 4 && "$max" == 1 && "$raw" == 114000 ]] || return 1
  else
    return 1
  fi
}
