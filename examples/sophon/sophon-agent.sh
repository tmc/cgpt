#!/bin/bash
# sophon-agent.sh - Sophon agent for human-machine collaboration
set -euo pipefail

# Configuration
export HIST_FILE=${HIST_FILE:-${H:-.h-mk}}
export CYCLE=${CYCLE:-0}
export ITERATIONS=${ITERATIONS:-10}
export SOPHON_PSTART="${SOPHON_PSTART:-.h-sophon-agent-init}"
export INIT_TOKENS="${INIT_TOKENS:-5000}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
GRAY='\033[1;30m'
NC='\033[0m' # No Color

# Logging functions
log_info() { echo -e "${GREEN}[INFO]${NC} $*" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*" >&2; }
log_debug() { echo -e "${GRAY}[DEBUG]${NC} $*" >&2; }

# Error handling
trap 'echo "Error on line $LINENO" >&2' ERR

# Utility Functions
trajectory_depth() {
    local depth=0
    local file="$1"
    if [ -f "${file}" ]; then
        depth=$(cat "${file}" | yq -r '.messages[] | select(.role == "human") | length')
    fi
    echo "${depth}"
}

cycle_finished() {
    if [[ -f .agent-finished || ${CYCLE} -ge ${ITERATIONS} ]]; then
        log_info "FINISHED!: CYCLE=${CYCLE} ITERATIONS=${ITERATIONS}"
        return 0
    else
        log_debug "NOT FINISHED!: CYCLE=${CYCLE} ITERATIONS=${ITERATIONS}"
        return 1
    fi
}

init() {
    log_info "Initializing Sophon agent..."
    echo -e "${GRAY}"
    echo "  HIST_DEPTH=$(trajectory_depth "user")"
    echo "  HIST_FILE=${HIST_FILE}"
    echo "  CYCLE=${CYCLE}"
    echo "  ITERATIONS=${ITERATIONS}"
    echo "  SOPHON_PSTART=${SOPHON_PSTART}"
    echo -e "${NC}"

    if [ -f "${HIST_FILE}" ]; then
        log_debug "Agent file exists: ${HIST_FILE}"
        return 0
    fi

    if [ ! -f "${SOPHON_PSTART}" ]; then
        log_warn "No start file found: ${SOPHON_PSTART}"
        return 1
    fi

    if [ -t 0 ]; then
        log_info "Starting with initial instructions from: ${SOPHON_PSTART}"
        cgpt -I "${SOPHON_PSTART}" -O "${HIST_FILE}" -t "${INIT_TOKENS}" -c=false
    else
        cat - | cgpt -I "${SOPHON_PSTART}" -O "${HIST_FILE}" -t "${INIT_TOKENS}"
    fi
}

run_cycle() {
    log_info "Running cycle ${CYCLE}..."

    # Show instructions
    if [ -f .agent-instructions ]; then
        log_info "Current instructions:"
        cat .agent-instructions
    fi

    # Run cgpt with input from instructions
    log_info "Running cgpt..."
    {
        echo "Instructions:"
        cat .agent-instructions
        echo -e "\nPlease implement these requirements in txtar format."
    } | cgpt -I "${HIST_FILE}" -O "${HIST_FILE}" -t 6400 -c=false || return 1

    # Apply txtar content
    log_info "Applying txtar content..."
    cat "${HIST_FILE}" | yq -P .messages[-1].text | txtar -x || return 1

    log_info "Cycle ${CYCLE} complete"
}

main() {
    init

    log_info "Starting Sophon agent..."
    log_debug "HIST_FILE=${HIST_FILE}"
    log_debug "CYCLE=${CYCLE}"
    log_debug "ITERATIONS=${ITERATIONS}"
    log_debug "SOPHON_PSTART=${SOPHON_PSTART}"
    log_debug "HIST_DEPTH=$(trajectory_depth 'human')"

    log_info "Running main loop..."

    while ! cycle_finished; do
        run_cycle
        export CYCLE=$((CYCLE + 1))
        log_info "Sleeping for 3 seconds..."
        sleep 3
    done
}

# Main execution
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    log_info "Running Sophon agent..."
    main "$@"
    log_info "Sophon agent finished."
fi 