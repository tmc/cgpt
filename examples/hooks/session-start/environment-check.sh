#!/bin/bash
# Session start hook to check environment and setup
# This hook ensures the environment is ready for cgpt usage

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_status() {
    local status=$1
    local message=$2
    case $status in
        "OK")
            echo -e "${GREEN}[OK]${NC} $message" >&2
            ;;
        "WARN")
            echo -e "${YELLOW}[WARN]${NC} $message" >&2
            ;;
        "ERROR")
            echo -e "${RED}[ERROR]${NC} $message" >&2
            ;;
    esac
}

# Read context from stdin
CONTEXT=$(cat)

# Extract backend from context if possible
BACKEND="unknown"
if command -v jq >/dev/null 2>&1; then
    BACKEND=$(echo "$CONTEXT" | jq -r '.config.backend // "unknown"' 2>/dev/null)
fi

echo_status "OK" "Starting cgpt session with backend: $BACKEND"

# Check for required directories
CGPT_DIR="$HOME/.cgpt"
if [ ! -d "$CGPT_DIR" ]; then
    mkdir -p "$CGPT_DIR"
    echo_status "OK" "Created cgpt directory: $CGPT_DIR"
else
    echo_status "OK" "cgpt directory exists: $CGPT_DIR"
fi

# Check for history directory
HISTORY_DIR="$CGPT_DIR/history"
if [ ! -d "$HISTORY_DIR" ]; then
    mkdir -p "$HISTORY_DIR/sessions"
    echo_status "OK" "Created history directory: $HISTORY_DIR"
fi

# Check for analysis directory
ANALYSIS_DIR="$CGPT_DIR/analysis"
if [ ! -d "$ANALYSIS_DIR" ]; then
    mkdir -p "$ANALYSIS_DIR"
    echo_status "OK" "Created analysis directory: $ANALYSIS_DIR"
fi

# Check for backups directory
BACKUP_DIR="$CGPT_DIR/backups"
if [ ! -d "$BACKUP_DIR" ]; then
    mkdir -p "$BACKUP_DIR"
    echo_status "OK" "Created backup directory: $BACKUP_DIR"
fi

# Check API key availability based on backend
case $BACKEND in
    "anthropic")
        if [ -z "$ANTHROPIC_API_KEY" ]; then
            echo_status "WARN" "ANTHROPIC_API_KEY not set - authentication may fail"
        else
            echo_status "OK" "Anthropic API key is configured"
        fi
        ;;
    "openai")
        if [ -z "$OPENAI_API_KEY" ]; then
            echo_status "WARN" "OPENAI_API_KEY not set - authentication may fail"
        else
            echo_status "OK" "OpenAI API key is configured"
        fi
        ;;
    "googleai")
        if [ -z "$GOOGLE_API_KEY" ]; then
            echo_status "WARN" "GOOGLE_API_KEY not set - authentication may fail"
        else
            echo_status "OK" "Google API key is configured"
        fi
        ;;
    "ollama")
        # Check if Ollama is running
        if curl -s http://localhost:11434/api/version >/dev/null 2>&1; then
            echo_status "OK" "Ollama service is running"
        else
            echo_status "WARN" "Ollama service may not be running on localhost:11434"
        fi
        ;;
esac

# Check disk space in cgpt directory
AVAILABLE_SPACE=$(df "$CGPT_DIR" | tail -1 | awk '{print $4}')
if [ "$AVAILABLE_SPACE" -lt 102400 ]; then  # Less than 100MB
    echo_status "WARN" "Low disk space available: ${AVAILABLE_SPACE}KB"
else
    echo_status "OK" "Sufficient disk space available"
fi

# Check for common utilities that hooks might need
for utility in jq python3 curl; do
    if command -v "$utility" >/dev/null 2>&1; then
        echo_status "OK" "$utility is available"
    else
        echo_status "WARN" "$utility is not available - some hooks may not work"
    fi
done

# Log session start
SESSION_LOG="$CGPT_DIR/session.log"
echo "$(date): Session started with backend $BACKEND" >> "$SESSION_LOG"

echo_status "OK" "Environment check completed"
exit 0