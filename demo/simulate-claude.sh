#!/bin/bash
# Simulates a Claude Code session for demo GIF recording
# Usage: ./simulate-claude.sh

# ─── Colors ───────────────────────────────────
BOLD='\033[1m'
DIM='\033[2m'
ITALIC='\033[3m'
RESET='\033[0m'
WHITE='\033[97m'
GRAY='\033[90m'
BLUE='\033[38;5;75m'
ORANGE='\033[38;5;208m'
GREEN='\033[38;5;114m'
YELLOW='\033[38;5;221m'
RED='\033[38;5;203m'
PURPLE='\033[38;5;141m'
CYAN='\033[38;5;80m'
BG_DARK='\033[48;5;236m'

# ─── Helpers ──────────────────────────────────
stream() {
    local text="$1"
    local speed="${2:-0.008}"
    for ((i=0; i<${#text}; i++)); do
        printf '%s' "${text:$i:1}"
        sleep "$speed"
    done
}

stream_line() {
    stream "$1" "${2:-0.008}"
    echo ""
}

prompt() {
    echo ""
    printf "${BLUE}${BOLD}❯${RESET} "
    stream "$1" 0.035
    echo ""
    sleep 0.5
    echo ""
}

tool_header() {
    printf "  ${DIM}⏵${RESET} ${PURPLE}$1${RESET}"
    echo ""
}

tool_result() {
    printf "    ${DIM}$1${RESET}"
    echo ""
}

divider() {
    printf "${DIM}─────────────────────────────────────────────────────${RESET}"
    echo ""
}

# ─── Welcome Banner ──────────────────────────
clear
echo ""
printf "${ORANGE}${BOLD}╭─────────────────────────────────────────────╮${RESET}\n"
printf "${ORANGE}${BOLD}│${RESET}  ${ORANGE}${BOLD}Claude Code${RESET}  ${DIM}v2.1  •  claude-opus-4-7${RESET}       ${ORANGE}${BOLD}│${RESET}\n"
printf "${ORANGE}${BOLD}│${RESET}  ${DIM}cwd: ~/my-project${RESET}                         ${ORANGE}${BOLD}│${RESET}\n"
printf "${ORANGE}${BOLD}╰─────────────────────────────────────────────╯${RESET}\n"
echo ""
printf "  ${DIM}Tips: /help for commands • /auto for Autopus${RESET}\n"
echo ""
sleep 1.5

# ─── Step 1: /auto plan ──────────────────────
prompt "/auto plan \"Add OAuth2 with Google and GitHub providers\""

printf "${ORANGE}${BOLD}🐙 Autopus ─────────────────────────${RESET}\n"
echo ""
sleep 0.3

stream_line "  Analyzing feature description..." 0.015
sleep 0.5

tool_header "Agent: spec-writer spawned"
sleep 0.3
tool_result "Reading go.mod, existing specs..."
sleep 0.4
tool_result "Scanning pkg/ for existing auth patterns..."
sleep 0.5

echo ""
stream_line "  ${GREEN}✓${RESET} PRD generated ${DIM}(Standard mode, 10 sections)${RESET}" 0.012
stream_line "  ${GREEN}✓${RESET} SPEC-AUTH-001 created" 0.012
echo ""

printf "  ${DIM}├── ${RESET}${WHITE}prd.md${RESET}          ${DIM}Product Requirements${RESET}\n"
sleep 0.15
printf "  ${DIM}├── ${RESET}${WHITE}spec.md${RESET}         ${DIM}EARS requirements (5 P0, 3 P1)${RESET}\n"
sleep 0.15
printf "  ${DIM}├── ${RESET}${WHITE}plan.md${RESET}         ${DIM}4 tasks → 3 executors${RESET}\n"
sleep 0.15
printf "  ${DIM}├── ${RESET}${WHITE}acceptance.md${RESET}   ${DIM}12 Given-When-Then criteria${RESET}\n"
sleep 0.15
printf "  ${DIM}└── ${RESET}${WHITE}research.md${RESET}     ${DIM}Risks + alternatives${RESET}\n"
echo ""
sleep 0.8

printf "  ${CYAN}Next:${RESET} /auto go SPEC-AUTH-001\n"
sleep 2

# ─── Step 2: /auto go ────────────────────────
prompt "/auto go SPEC-AUTH-001 --auto --loop"

printf "${ORANGE}${BOLD}🐙 Pipeline ─────────────────────────────────────────${RESET}\n"
echo ""
sleep 0.3

# Phase 1
printf "  ${DIM}◌${RESET} Phase 1: Planning\r"
sleep 0.6
printf "  ${GREEN}✓${RESET} Phase 1:   ${WHITE}Planning${RESET}         ${DIM}planner decomposed 4 tasks${RESET}\n"
sleep 0.3

# Phase 1.5
printf "  ${DIM}◌${RESET} Phase 1.5: Test Scaffold\r"
sleep 0.8
printf "  ${GREEN}✓${RESET} Phase 1.5: ${WHITE}Test Scaffold${RESET}    ${DIM}8 failing tests created (RED)${RESET}\n"
sleep 0.3

# Phase 2
printf "  ${DIM}◌${RESET} Phase 2:   Implementation (3 executors)...\r"
sleep 1.2
printf "  ${GREEN}✓${RESET} Phase 2:   ${WHITE}Implementation${RESET}   ${DIM}3 executors in parallel worktrees${RESET}\n"
sleep 0.3

# Phase 2.5
printf "  ${DIM}◌${RESET} Phase 2.5: Annotation\r"
sleep 0.5
printf "  ${GREEN}✓${RESET} Phase 2.5: ${WHITE}Annotation${RESET}       ${DIM}@AX tags applied to 6 files${RESET}\n"
sleep 0.3

# Phase 3
printf "  ${DIM}◌${RESET} Phase 3:   Testing\r"
sleep 0.8
printf "  ${GREEN}✓${RESET} Phase 3:   ${WHITE}Testing${RESET}          ${DIM}coverage: 58%% → 89%%${RESET}\n"
sleep 0.3

# Phase 4
printf "  ${DIM}◌${RESET} Phase 4:   Review\r"
sleep 1.0
printf "  ${GREEN}✓${RESET} Phase 4:   ${WHITE}Review${RESET}           ${DIM}TRUST 5: APPROVE │ Security: PASS${RESET}\n"
sleep 0.3

printf "  ${DIM}───────────────────────────────────────────────────${RESET}\n"
printf "  ${GREEN}${BOLD}✅ 4/4 tasks${RESET} │ ${GREEN}89%% coverage${RESET} │ ${GREEN}0 security issues${RESET} │ ${DIM}3m 47s${RESET}\n"
sleep 2

# ─── Step 3: /auto sync ──────────────────────
prompt "/auto sync SPEC-AUTH-001"

sleep 0.5
stream_line "  Updating SPEC status → ${GREEN}completed${RESET}" 0.015
sleep 0.3
stream_line "  Syncing ARCHITECTURE.md..." 0.015
sleep 0.3
stream_line "  Updating project docs..." 0.015
sleep 0.5

echo ""
printf "${GREEN}${BOLD}╭────────────────────────────────────────╮${RESET}\n"
printf "${GREEN}${BOLD}│${RESET} ${ORANGE}🐙${RESET} ${BOLD}Pipeline Complete!${RESET}                  ${GREEN}${BOLD}│${RESET}\n"
printf "${GREEN}${BOLD}│${RESET}                                        ${GREEN}${BOLD}│${RESET}\n"
printf "${GREEN}${BOLD}│${RESET}  SPEC-AUTH-001: OAuth2 Authentication  ${GREEN}${BOLD}│${RESET}\n"
printf "${GREEN}${BOLD}│${RESET}  Tasks: ${GREEN}4/4${RESET}  │  Coverage: ${GREEN}89%%${RESET}         ${GREEN}${BOLD}│${RESET}\n"
printf "${GREEN}${BOLD}│${RESET}  Review: ${GREEN}${BOLD}APPROVE${RESET}                       ${GREEN}${BOLD}│${RESET}\n"
printf "${GREEN}${BOLD}│${RESET}                                        ${GREEN}${BOLD}│${RESET}\n"
printf "${GREEN}${BOLD}╰────────────────────────────────────────╯${RESET}\n"
echo ""

sleep 4
