#!/usr/bin/env bash
# prompt-eval.sh - Prompt evaluation and comparison tools
#
# This script provides tools for evaluating prompt effectiveness and
# comparing different prompt versions.
#
# Features:
# - Multiple evaluation criteria
# - Comparative analysis
# - Improvement tracking
# - Metric-based assessment
#
# Usage:
#   ./prompt-eval.sh <prompt-file> <output-file>
#   ./prompt-eval.sh compare <prompt1> <prompt2> <output>
set -euo pipefail

# Evaluation criteria
EVAL_CRITERIA=(
    "Clarity"
    "Completeness"
    "Error handling"
    "Meta-cognitive elements"
    "Self-improvement capability"
)

# Function to evaluate a prompt
evaluate_prompt() {
    local prompt_file="$1"
    local output_file="$2"
    
    cgpt -s "You are a prompt evaluation expert. Evaluate this prompt against these criteria:
$(printf '%s\n' "${EVAL_CRITERIA[@]}" | sed 's/^/- /')

Output Format:
<evaluation>
  <scores>
    <criterion name='NAME'>SCORE</criterion>
    ...
  </scores>
  <analysis>Detailed analysis</analysis>
  <recommendations>Improvement suggestions</recommendations>
</evaluation>" \
         -f "${prompt_file}" \
         -O "${output_file}" \
         -p '<evaluation>'
}

# Function to compare prompts
compare_prompts() {
    local prompt1="$1"
    local prompt2="$2"
    local output_file="$3"
    
    cgpt -s "You are a prompt comparison expert. Compare these two prompts and analyze their strengths and weaknesses.

Output Format:
<comparison>
  <differences>Key differences</differences>
  <strengths-prompt1>Strengths of first prompt</strengths-prompt1>
  <strengths-prompt2>Strengths of second prompt</strengths-prompt2>
  <recommendations>Suggested improvements</recommendations>
</comparison>" \
         -f "${prompt1}" "${prompt2}" \
         -O "${output_file}" \
         -p '<comparison>'
}