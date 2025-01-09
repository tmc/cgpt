#!/bin/bash
# sophon-agent.sh - Sophon agent for human-machine collaboration.
set -euo pipefail

HIST_FILE=${HIST_FILE:-${H:-.h-mk}}
CYCLE=${CYCLE:-0}
ITERATIONS=${ITERATIONS:-10}
SOPHON_PSTART="${SOPHON_PSTART:-.h-sophon-agent-init}"

main() {
  init
  echo "Starting Sophon agent..."
  echo "HIST_FILE=${HIST_FILE}"
  echo "CYCLE=${CYCLE}"
  echo "ITERATIONS=${ITERATIONS}"
  echo "SOPHON_PSTART=${SOPHON_PSTART}"
  echo "HIST_DEPTH=$(trajectory_depth 'human')"

  echo "Running main loop..."

  set -x
  cf=$(cycle_finished)
  until [ "$cf" -eq 0 ]; do
    (
    if [ "$CYCLE" -eq 0 ]; then
      echo 'This is how you are running:';
      ctx-exec cat "${BASH_SOURCE[0]}";
      echo 'Orient yourself and execute on goals';
      echo 'Place a .agent-finished file to stop the loop, populate it with something useful';
      echo "You have some influence on this loop's behavior."
    fi
    (
      echo 'Files modified from your last response:';
      cat "${HIST_FILE:-.h-mk}" | yq -P .messages[-1].text | txtar -list
    );
    (if [ -f .agent-shell-request.log ]; then
      echo -e "\033[1;33mShell results from your last run:\033[0m";
      ctx-exec -tag=shell-exec-result cat .agent-shell-request.log

      mv .agent-shell-request.log .agent-shell-request.log.$$."$CYCLE"
      fi);
    ctx-exec -tag=agent-intructions cat .agent-instructions;
    #ctx-exec -tag=extra-agent-context bash .extra-agent-conext.sh;
    ctx-exec -tag=git-context "git status .; git log -10 --pretty=format:'%h %ar %d: %s' --shortstat";
    ctx-exec go vet ./...; ctx-exec go test -cover ./...) | tee .agent-input-ctx | cgpt -I "${HIST_FILE:-h-mk}" -O "${HIST_FILE:-h-mk}" -t 6400 || break;

    # Apply txtar content
    # TODO: failure here should get information back to the loop, and possibly user if needed
    cat "${HIST_FILE:-h-mk}" | yq -P .messages[-1].text | txtar -x

    # Check for shell request
    if [ -f .agent-shell-request.sh ]; then
      echo -e "\033[1;33mAgent requested to run shell commands:\033[0m"
      cat .agent-shell-request.sh
      echo -e "\033[1;36mPress Enter within 10 seconds to execute these commands, or Ctrl+C for interactive mode:\033[0m"
      if ! read -r -t 10; then
        echo -e "\033[1;32mAuto-accepting after timeout...\033[0m"
      fi
      # Execute commands if not skipped in interactive mode
      bash .agent-shell-request.sh 2>&1 | tee .agent-shell-request.log | tee -a .agent-shell-request-history.log
      rm .agent-shell-request.sh
    fi

    echo
    (date | txtar .summary-with-reflection-and-metalearning.txt) | tee -a .agent-summary-log.txt
    date
    echo "Sleeping for 10 seconds..."
    sleep 10
    [ "${CYCLE}" -ge "${ITERATIONS}" ] && { echo "Max iterations (${ITERATIONS}) reached"; break; }
    echo "Iteration $((++CYCLE))/$ITERATIONS $(date)"
  done
}

# Check if agent cycle is finished based on .agent-finished file or CYCLE/ITERATIONS
# Returns 0 if finished, 1 if should continue
cycle_finished() {
  local cycle=${CYCLE:-0}
  local iterations=${ITERATIONS:-1}

  if [[ -f .agent-finished || $cycle -ge $iterations ]]; then
    printf 'FINISHED!: CYCLE=%d ITERATIONS=%d\n' "$cycle" "$iterations" >&2
    echo 0
  else
    printf 'NOT FINISHED!: CYCLE=%d ITERATIONS=%d\n' "$cycle" "$iterations" >&2  
    echo 1
  fi
}

init() {
  echo "Initializing Sophon agent..."
  echo -e "\033[1;30m"
  echo "  HIST_DEPTH=$(trajectory_depth "user")"
  echo "  HIST_FILE=${HIST_FILE}"
  echo "  CYCLE=${CYCLE}"
  echo "  ITERATIONS=${ITERATIONS}"
  echo "  SOPHON_PSTART=${SOPHON_PSTART}"
  if [ -f "${HIST_FILE}" ]; then
    echo "Agent file exists: ${HIST_FILE}"
    else
      if [ -f "${SOPHON_PSTART}" ]; then
        # if not a tty, use stdin and suggest next iteration
        if [ -t 0 ]; then
          echo "Starting with initial instructions from: ${SOPHON_PSTART}"
          set -x
          cgpt -I "${SOPHON_PSTART}" -O "${HIST_FILE}" -t "${INIT_TOKEN:-5000}" -c=false
          set +x
        else
          # send stdin into cgpt:
          infilter=$(hook agent-infilter.sh "cat")
          cat - | "${infilter}" | cgpt -I "${SOPHON_PSTART}" -O "${HIST_FILE}" -t "${INIT_TOKENS:-5000}"
        fi
      # if [ -t 0 ]; then
      #   echo "(Optional) provide initial instructions for the agent:"
      # fi
    fi
  fi
}

trajectory_depth() {
  local depth=0
  local file="$1"
  if [ -f "${file}" ]; then
    depth=$(cat "${file}" | yq -r '.messages[] | select(.role == "human") | length')
  fi
  echo "${depth}"
}

function hook() {
  local hook_name="$1"
  local default="${2:-}"
  git_dir=$(git rev-parse --show-toplevel 2>/dev/null)
  if [ -n "$git_dir" ]; then
    hook_dir="$git_dir/.agent/hooks"
  fi
  hook="${hook_dir}/${hook_name}"
  if [ -f "${hook}" ]; then
     echo "Running hook: ${hook}"
     export PS4='+sophon(${BASH_SOURCE}:${LINENO}): ${FUNCNAME[0]:+${FUNCNAME[0]}(): }'
     bash "$hook"
   elif [ -n "${default}" ]; then
     echo "${default}"
  fi
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  HIST_DEPTH=$(trajectory_depth 'human')
  
  # Print startup info on first run
  if [ "${CYCLE:-0}" -eq 0 ] && [ "${HIST_DEPTH:-0}" -eq 0 ]; then
    printf "Starting agent loop...\n"
    printf "HIST_FILE=%s\n" "${HIST_FILE}"
    printf "CYCLE=%d\n" "${CYCLE:-0}"
    printf "ITERATIONS=%d\n" "${ITERATIONS:-0}"
    if [ -f .agent-finished ]; then
      # move to history:
      mv .agent-finished .agent-finished.prev.$$
    fi
  fi

  # add nice colors:
  GREEN='\033[0;32m'
  NC='\033[0m' # No Color

  echo -e "${GREEN}Running Sophon agent...${NC}"
  main "$@" 
  ret=$?
  echo -e "${GREEN}Sophon agent finished.${NC}"
  exit $ret
fi

if test -f .agent-finished ; then
  echo "Agent loop finished"
  cp .agent-finished ".agent-finished.$PPID.$$.$CYCLE"
  hook agent-finished-pre

  cat .agent-finished
  if [ -t 0 ]; then
    echo "Press Enter to continue..."
    read -r
  fi
  # Add a final summary
  hook agent-finished
  hook agent-finished-post
fi
