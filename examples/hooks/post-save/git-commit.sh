#!/bin/bash
# Post-save hook to automatically commit conversation history to git
# This hook demonstrates integration with version control

# Read context from stdin
CONTEXT=$(cat)

# Extract information from context
HISTORY_FILE=""
if command -v jq >/dev/null 2>&1; then
    HISTORY_FILE=$(echo "$CONTEXT" | jq -r '.metadata.historyFile // ""' 2>/dev/null)
fi

# If we don't have the history file from context, try to find it
if [ -z "$HISTORY_FILE" ] || [ "$HISTORY_FILE" = "null" ]; then
    # Look for recently modified .yaml files in the cgpt directory
    CGPT_DIR="$HOME/.cgpt"
    if [ -d "$CGPT_DIR" ]; then
        HISTORY_FILE=$(find "$CGPT_DIR" -name "*.yaml" -type f -mmin -1 | head -1)
    fi
fi

# If still no file, exit silently
if [ -z "$HISTORY_FILE" ] || [ ! -f "$HISTORY_FILE" ]; then
    echo "No history file to commit" >&2
    exit 0
fi

# Get the directory containing the history file
HISTORY_DIR=$(dirname "$HISTORY_FILE")

# Check if the history directory is a git repository
if [ ! -d "$HISTORY_DIR/.git" ]; then
    # Initialize git repository if it doesn't exist
    cd "$HISTORY_DIR" || exit 1
    if git init >/dev/null 2>&1; then
        echo "Initialized git repository in $HISTORY_DIR" >&2
        # Set up .gitignore for common patterns
        cat > .gitignore << EOF
# Temporary files
*.tmp
*.temp
.DS_Store
Thumbs.db

# Logs
*.log

# Sensitive files (just in case)
*key*
*secret*
*password*
EOF
        git add .gitignore >/dev/null 2>&1
        git commit -m "Initial commit with gitignore" >/dev/null 2>&1
    else
        echo "Failed to initialize git repository" >&2
        exit 1
    fi
else
    cd "$HISTORY_DIR" || exit 1
fi

# Check if the file is tracked or needs to be added
RELATIVE_FILE=$(realpath --relative-to="$HISTORY_DIR" "$HISTORY_FILE" 2>/dev/null || basename "$HISTORY_FILE")

if ! git ls-files --error-unmatch "$RELATIVE_FILE" >/dev/null 2>&1; then
    # File is not tracked, add it
    git add "$RELATIVE_FILE" >/dev/null 2>&1
    COMMIT_TYPE="Add"
else
    # File is tracked, stage changes
    git add "$RELATIVE_FILE" >/dev/null 2>&1
    COMMIT_TYPE="Update"
fi

# Check if there are changes to commit
if git diff --staged --quiet; then
    echo "No changes to commit for $RELATIVE_FILE" >&2
    exit 0
fi

# Extract some context for the commit message
MESSAGE_COUNT="unknown"
BACKEND="unknown"
MODEL="unknown"

if command -v jq >/dev/null 2>&1; then
    MESSAGE_COUNT=$(echo "$CONTEXT" | jq -r '.messages | length // "unknown"' 2>/dev/null)
    BACKEND=$(echo "$CONTEXT" | jq -r '.config.backend // "unknown"' 2>/dev/null)
    MODEL=$(echo "$CONTEXT" | jq -r '.config.model // "unknown"' 2>/dev/null)
fi

# Create a descriptive commit message
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
COMMIT_MSG="$COMMIT_TYPE conversation history

- File: $RELATIVE_FILE
- Messages: $MESSAGE_COUNT
- Backend: $BACKEND
- Model: $MODEL
- Timestamp: $TIMESTAMP"

# Commit the changes
if git commit -m "$COMMIT_MSG" >/dev/null 2>&1; then
    SHORT_HASH=$(git rev-parse --short HEAD)
    echo "Committed conversation history: $SHORT_HASH" >&2
    echo "  File: $RELATIVE_FILE" >&2
    echo "  Messages: $MESSAGE_COUNT" >&2
else
    echo "Failed to commit conversation history" >&2
    exit 1
fi

exit 0