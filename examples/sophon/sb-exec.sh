#!/usr/bin/env bash
# sb-exec.sh - Execute commands in a Docker container and save output as git notes
#
# Usage: ./sb-exec.sh [options] <command>
#   -i, --image IMAGE     Docker image to use (default: ubuntu:latest)
#   -w, --workdir DIR     Working directory inside container (default: /workspace)
#   -v, --volume DIR      Mount local directory (default: current directory)
#   -t, --tag TAG         Git note tag name (default: sb-exec)
#   -m, --message MSG     Message to include with git note
#   -n, --no-notes        Don't create git notes, just execute command
#   -h, --help            Show this help message
#
# Examples:
#   ./sb-exec.sh "ls -la"
#   ./sb-exec.sh --image python:3.9 "python -m pip list"
#   ./sb-exec.sh --tag "performance-test" "time python benchmark.py"

set -euo pipefail

# Default values
DOCKER_IMAGE="ubuntu:latest"
WORKDIR="/workspace"
VOLUME="$(pwd)"
GIT_NOTE_TAG="sb-exec"
GIT_NOTE_MSG=""
CREATE_NOTES=true
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
OUTPUT_FILE="/tmp/sb-exec-output-${TIMESTAMP}.txt"

# Function to display usage information
usage() {
  grep "^# " "$0" | cut -c 3-
  exit 0
}

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -i|--image)
      DOCKER_IMAGE="$2"
      shift 2
      ;;
    -w|--workdir)
      WORKDIR="$2"
      shift 2
      ;;
    -v|--volume)
      VOLUME="$2"
      shift 2
      ;;
    -t|--tag)
      GIT_NOTE_TAG="$2"
      shift 2
      ;;
    -m|--message)
      GIT_NOTE_MSG="$2"
      shift 2
      ;;
    -n|--no-notes)
      CREATE_NOTES=false
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      break
      ;;
  esac
done

# Check if command was provided
if [[ $# -lt 1 ]]; then
  echo "Error: No command specified"
  usage
fi

# Combine remaining arguments as the command to execute
COMMAND="$*"

# Ensure we're in a git repository if creating notes
if [[ "$CREATE_NOTES" == true ]]; then
  if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    echo "Error: Not inside a git repository. Use --no-notes to skip git notes creation."
    exit 1
  fi
fi

# Create a unique container name
CONTAINER_NAME="sb-exec-$(date +%s)"

echo "=================================="
echo "🚀 sb-exec execution"
echo "=================================="
echo "• Image:     $DOCKER_IMAGE"
echo "• Command:   $COMMAND"
echo "• Workdir:   $WORKDIR"
echo "• Volume:    $VOLUME"
echo "• Time:      $(date)"
if [[ "$CREATE_NOTES" == true ]]; then
  echo "• Git note:  $GIT_NOTE_TAG"
fi
echo "=================================="

# Create header for output file
{
  echo "=================================="
  echo "sb-exec execution - ${TIMESTAMP}"
  echo "=================================="
  echo "Image:     $DOCKER_IMAGE"
  echo "Command:   $COMMAND"
  echo "Workdir:   $WORKDIR"
  echo "Volume:    $VOLUME mounted at $WORKDIR"
  echo "Time:      $(date)"
  if [[ -n "$GIT_NOTE_MSG" ]]; then
    echo "Message:   $GIT_NOTE_MSG"
  fi
  echo "=================================="
  echo
  echo "COMMAND OUTPUT:"
  echo "--------------------------------"
} > "$OUTPUT_FILE"

# Execute command in Docker container and capture output
echo "⏳ Running command in Docker container..."
{
  set -o pipefail
  docker run --rm --name "$CONTAINER_NAME" \
    -v "$VOLUME:$WORKDIR" \
    -w "$WORKDIR" \
    "$DOCKER_IMAGE" \
    /bin/sh -c "$COMMAND" 2>&1 | tee -a "$OUTPUT_FILE"
} || {
  EXIT_CODE=$?
  echo "❌ Command failed with exit code $EXIT_CODE" | tee -a "$OUTPUT_FILE"
}

# Add footer with execution time
{
  echo
  echo "--------------------------------"
  echo "Execution completed at: $(date)"
  echo "=================================="
} >> "$OUTPUT_FILE"

# Get the current commit hash
if [[ "$CREATE_NOTES" == true ]]; then
  COMMIT_HASH=$(git rev-parse HEAD)
  
  echo "📝 Attaching output as git note to commit $COMMIT_HASH..."
  # Add the script output as a git note
  git notes --ref="$GIT_NOTE_TAG" add -f -F "$OUTPUT_FILE" "$COMMIT_HASH"
  
  # Show confirmation
  echo "✅ Git note created with tag '$GIT_NOTE_TAG'"
  echo "   View with: git notes --ref='$GIT_NOTE_TAG' show $COMMIT_HASH"
fi

# Display summary
echo
echo "=================================="
echo "✅ Execution complete"
echo "• Output saved to: $OUTPUT_FILE"
if [[ "$CREATE_NOTES" == true ]]; then
  echo "• Git note created with tag: $GIT_NOTE_TAG"
fi
echo "=================================="