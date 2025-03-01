#!/usr/bin/env bash
# meta-cognitive.sh - Pattern for adding meta-cognitive capabilities to prompts
#
# This script provides patterns for adding meta-cognitive capabilities
# to prompts, enabling better self-awareness and reasoning.
#
# Features:
# - Role awareness
# - Reasoning patterns
# - State tracking
# - Decision explanation
#
# Usage:
#   ./meta-cognitive.sh <input-prompt> <output-file>

BASE_METACOG='<meta-cognition>
<available-roles>
[List relevant roles]
</available-roles>
<active-role>[Current primary role]</active-role>
<reasoning-pattern>
[Current thinking process]
</reasoning-pattern>
<current-state>
[Current operational state]
</current-state>
</meta-cognition>'

add_metacognitive() {
    local prompt_file="$1"
    local output_file="$2"
    
    cgpt -s "You are a meta-cognitive enhancement expert. Add meta-cognitive capabilities to this prompt using this structure:
${BASE_METACOG}

Ensure the additions are relevant and meaningful." \
         -f "${prompt_file}" \
         -O "${output_file}" \
         -p '<meta-cognition>'
}

