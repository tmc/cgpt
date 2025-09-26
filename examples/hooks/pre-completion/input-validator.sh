#!/bin/bash
# Pre-completion hook to validate input length and content
# This hook demonstrates input validation before sending to the LLM

# Read the hook context from stdin
CONTEXT=$(cat)

# Extract the last message using jq (if available) or basic parsing
if command -v jq >/dev/null 2>&1; then
    MESSAGE_COUNT=$(echo "$CONTEXT" | jq -r '.messages | length // 0')
    LAST_MESSAGE=$(echo "$CONTEXT" | jq -r '.messages[-1].parts[0] // ""' 2>/dev/null)
else
    # Fallback for systems without jq
    MESSAGE_COUNT=$(echo "$CONTEXT" | grep -c '"role"' || echo "0")
    LAST_MESSAGE="$CONTEXT"
fi

# Check if message is too short (might be accidental)
if [ ${#LAST_MESSAGE} -lt 5 ]; then
    echo "WARNING: Input message is very short (${#LAST_MESSAGE} characters). Are you sure you want to continue?" >&2
    # For demo purposes, we'll allow it but warn
    exit 0
fi

# Check if message is extremely long (might hit token limits)
if [ ${#LAST_MESSAGE} -gt 50000 ]; then
    echo "ERROR: Input message is very long (${#LAST_MESSAGE} characters). This may exceed token limits." >&2
    exit 1
fi

# Check for potential prompt injection patterns
if echo "$LAST_MESSAGE" | grep -qE "(ignore previous instructions|disregard|override|forget everything)"; then
    echo "WARNING: Potential prompt injection detected. Review your input carefully." >&2
    # For demo, we warn but continue. In production, you might abort.
    exit 0
fi

echo "Input validation passed: ${MESSAGE_COUNT} messages, last message ${#LAST_MESSAGE} characters" >&2
exit 0