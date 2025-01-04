#!/bin/bash
# dialogue-proper.sh

# Colors
CYAN='\033[1;36m'
MAGENTA='\033[1;35m'
YELLOW='\033[1;33m'
RESET='\033[0m'

# Setup history
mkdir -p .cgpt-dialogue
PSYCH_HISTORY=".cgpt-dialogue/psycho-history"
PRAGMA_HISTORY=".cgpt-dialogue/pragma-history"

# Clear and show header
clear
echo -e "${YELLOW}=== AI Dialogue Loop - Pipe in content to start! ===${RESET}\n"

# Read from stdin
while IFS= read -r input; do
    # PsychoPrompt processes input
    echo -e "${CYAN}🧠 PsychoPrompt responding to: '$input'${RESET}"

    # Run PsychoPrompt with proper history management
    echo "$input" | cgpt -s "you are PsychoPrompt - the psychological pattern analyzer.
    Respond briefly (2-3 sentences).
    Use <psych-insight> tags." \
        -I "$PSYCH_HISTORY" -O "$PSYCH_HISTORY" | \
    while read -r line; do
        echo -e "${CYAN}🧠 $line${RESET}"
        # Capture for PragmaCore's context
        echo "$line" > .cgpt-dialogue/last_psych_response
    done

    sleep 1

    # PragmaCore responds to PsychoPrompt
    echo -e "${MAGENTA}⚡ PragmaCore responding...${RESET}"

    # Feed PsychoPrompt's response to PragmaCore
    cat .cgpt-dialogue/last_psych_response | \
    cgpt -s "you are PragmaCore - the implementation architect.
    Read PsychoPrompt's response and build on it briefly (2-3 sentences).
    Use <impl-pattern> tags." \
        -I "$PRAGMA_HISTORY" -O "$PRAGMA_HISTORY" | \
    while read -r line; do
        echo -e "${MAGENTA}⚡ $line${RESET}"
    done

    echo -e "${YELLOW}---${RESET}\n"
done
