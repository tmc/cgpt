#!/bin/bash
# dialogue-ctx.sh

# Colors
CYAN='\033[1;36m'
MAGENTA='\033[1;35m'
YELLOW='\033[1;33m'
RESET='\033[0m'

# Setup history files
mkdir -p .cgpt-dialogue
PSYCH_HISTORY=".cgpt-dialogue/psycho-history"
PRAGMA_HISTORY=".cgpt-dialogue/pragma-history"

# Clear and show header
clear
echo -e "${YELLOW}=== AI Dialogue with ctx-exec wrapping ===${RESET}\n"

# Read all input at once
input=$(cat)

# Create the full dialogue command
ctx-exec "
# Initial PsychoPrompt response
echo '$input' | cgpt -s 'you are PsychoPrompt - the psychological pattern analyzer.
Respond briefly (2-3 sentences). Use <psych-insight> tags.' \
    -O '$PSYCH_HISTORY'

# Main dialogue loop
for turn in {1..5}; do
    echo '=== Turn \$turn ==='

    # PragmaCore responds
    cgpt -s 'you are PragmaCore - the implementation architect.
    Read the conversation history and build on it briefly (2-3 sentences).
    Use <impl-pattern> tags.' \
        -I '$PSYCH_HISTORY' -O '$PRAGMA_HISTORY'

    sleep 1

    # PsychoPrompt responds
    cgpt -s 'you are PsychoPrompt - the psychological pattern analyzer.
    Read the conversation history and build on it briefly (2-3 sentences).
    Use <psych-insight> tags.' \
        -I '$PRAGMA_HISTORY' -O '$PSYCH_HISTORY'

    sleep 1
done" | \
while read -r line; do
    if [[ $line == *"PsychoPrompt"* ]] || [[ $line =~ \<psych-insight\> ]]; then
        echo -e "${CYAN}🧠 $line${RESET}"
    elif [[ $line == *"PragmaCore"* ]] || [[ $line =~ \<impl-pattern\> ]]; then
        echo -e "${MAGENTA}⚡ $line${RESET}"
    elif [[ $line == "==="* ]]; then
        echo -e "\n${YELLOW}$line${RESET}\n"
    else
        echo -e "$line"
    fi
done
