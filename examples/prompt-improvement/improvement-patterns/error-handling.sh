#!/usr/bin/env bash
# error-handling.sh - Pattern for improving error handling in prompts
#
# This script provides patterns and tools for improving prompt
# error handling capabilities.
#
# Features:
# - Input validation
# - Error recovery
# - Fallback behaviors
# - Clear error messages
#
# Usage:
#   ./error-handling.sh <input-prompt> <output-file>

add_error_handling() {
    local prompt_file="$1"
    local output_file="$2"
    
    cgpt -s "You are an error handling expert. Improve this prompt's error handling capabilities by adding:
1. Input validation
2. Error recovery strategies
3. Fallback behaviors
4. Clear error messages

Output the improved prompt with these additions." \
         -f "${prompt_file}" \
         -O "${output_file}" \
         -p '<improved-prompt>'
}

