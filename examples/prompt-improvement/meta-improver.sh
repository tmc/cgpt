#!/usr/bin/env bash
# meta-improver.sh - Framework for systematic prompt improvement
#
# This script provides tools and patterns for enhancing prompts through
# systematic analysis, testing, and iteration.
#
# Features:
# - Prompt analysis
# - Improvement suggestions
# - Automated testing
# - Evaluation metrics
#
# Usage:
#   ./meta-improver.sh <input-prompt> [iterations]
#
# Example:
#   ./meta-improver.sh my-prompt.txt 3
set -euo pipefail

ts="$(date +%s)"

# Base prompt for improvement analysis
BASE_PROMPT='You are an expert prompt improvement system. Analyze prompts for:
1. Clarity and structure
2. Error handling
3. Meta-cognitive elements
4. Self-improvement capabilities

Output Format:
<prompt-analysis>
  <structure>Current prompt structure analysis</structure>
  <improvements>Suggested improvements</improvements>
  <rationale>Reasoning behind changes</rationale>
</prompt-analysis>

Provide concrete examples and improvements.'

# Function to analyze a prompt
analyze_prompt() {
    local prompt_file="$1"
    local output_file="$2"
    
    cgpt -s "${BASE_PROMPT}" \
         -i "Analyze this prompt for improvement:" \
         -f "${prompt_file}" \
         -O "${output_file}" \
         -p '<prompt-analysis>'
}

# Function to test prompt improvements
test_prompt() {
    local original="$1"
    local improved="$2"
    local test_case="$3"
    
    echo "Testing original prompt..."
    cgpt -s "$(cat "${original}")" -i "${test_case}" -O "original_result.txt"
    
    echo "Testing improved prompt..."
    cgpt -s "$(cat "${improved}")" -i "${test_case}" -O "improved_result.txt"
    
    # Compare results
    cgpt -s "You are a prompt evaluation expert. Compare these two responses and analyze the improvements:" \
         -f "original_result.txt" "improved_result.txt" \
         -O "comparison_${ts}.txt"
}

# Main improvement loop
main() {
    local prompt_file="$1"
    local iterations="${2:-3}"
    
    # Copy input prompt to working file
    cp "${prompt_file}" "current_prompt.txt"
    
    for i in $(seq 1 "${iterations}"); do
        echo "Improvement iteration $i..."
        analyze_prompt "current_prompt.txt" "improved_${i}.txt"
        test_prompt "current_prompt.txt" "improved_${i}.txt" "Sample test case"
        cp "improved_${i}.txt" "current_prompt.txt"
    done
}

# If script is run directly (not sourced), execute main function with arguments
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    if [[ $# -lt 1 ]]; then
        echo "Usage: $0 <prompt-file> [iterations]"
        exit 1
    fi
    main "$@"
fi