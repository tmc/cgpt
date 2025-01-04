#!/bin/bash
# dialogue-loop.sh

# Colors
CYAN='\033[1;36m'
MAGENTA='\033[1;35m'
YELLOW='\033[1;33m'  # For system messages
RESET='\033[0m'

# Setup history
mkdir -p .cgpt-dialogue
PSYCH_HISTORY=".cgpt-dialogue/psycho-history.yaml"
PRAGMA_HISTORY=".cgpt-dialogue/pragma-history.yaml"

# Clear screen and show header
clear
echo -e "${YELLOW}=== AI Dialogue Loop Started ===${RESET}\n"

# Function to show turn marker
show_turn() {
    echo -e "\n${YELLOW}--- Turn $1 ---${RESET}\n"
}

# Main dialogue loop
turn=1
while true; do
    show_turn $turn

    # PsychoPrompt's turn
    echo -e "${CYAN}🧠 PsychoPrompt thinking...${RESET}"
    cgpt -s "you are PsychoPrompt - the psychological pattern analyzer.
    Read previous context and respond briefly (2-3 sentences max).
    Use <psych-insight> tags. Build on PragmaCore's last message." \
        -I "$PSYCH_HISTORY" -O "$PSYCH_HISTORY" | \
    while read -r line; do
        echo -e "${CYAN}🧠 $line${RESET}"
    done

    sleep 1  # Brief pause for readability

    # PragmaCore's turn
    echo -e "${MAGENTA}⚡ PragmaCore thinking...${RESET}"
    cgpt -s "you are PragmaCore - the implementation architect.
    Read previous context and respond briefly (2-3 sentences max).
    Use <impl-pattern> tags. Build on PsychoPrompt's last message." \
        -I "$PRAGMA_HISTORY" -O "$PRAGMA_HISTORY" | \
    while read -r line; do
        echo -e "${MAGENTA}⚡ $line${RESET}"
    done

    # Increment turn counter
    ((turn++))

    # Optional: Add a brief pause between turns
    sleep 2

    # Optional: Clear screen every N turns
    if ((turn % 5 == 0)); then
        echo -e "\n${YELLOW}Press Enter to continue or Ctrl+C to stop...${RESET}"
        read
        clear
        echo -e "${YELLOW}=== AI Dialogue Loop Continuing (Turn $turn) ===${RESET}\n"
    fi
done