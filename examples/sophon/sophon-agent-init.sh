#!/bin/bash
# sophon-agent-init.sh - Initialize agent instructions based on codebase analysis
# Usage: ./sophon-agent-init.sh "Your goal description here"

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $*" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*" >&2; }

# Check if a goal was provided
if [ $# -lt 1 ]; then
    log_warn "Usage: $0 'Your goal description here'"
    exit 1
fi

GOAL="$1"
PROJECT_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)

# Gather project context
gather_context() {
    (
        set -x
        PS4=">> command: "
        # Project structure
        find . -type f | grep -v "node_modules\|\.git" | sort
        
        # Git context if available
        if git rev-parse --is-inside-work-tree &>/dev/null; then
            git branch
            git status .
            git log --stat -5 . | head -n 500
        fi
        
        # Language-specific context
        if [ -f "go.mod" ]; then
            cat go.mod
        fi
        if [ -f "package.json" ]; then
            cat package.json
        fi
        if [ -f "requirements.txt" ]; then
            cat requirements.txt
        fi
        
        # Goal
        echo "Goal: ${GOAL}"
    ) 2>&1 > .agent-context
}

# Main execution
log_info "Gathering project context..."
gather_context

log_info "Analyzing context and generating instructions..."
cat .agent-context | cgpt -s "You are an expert software development assistant. Based on the provided command output, analyze the project structure and create appropriate agent instructions. The instructions should be in a format suitable for an AI agent to implement the specified goal. Include relevant context from the codebase and specific requirements based on the goal. Output should be in plain text format suitable for .agent-instructions file." -O .agent-instructions

if [ -f ".agent-instructions" ]; then
    log_info "Created .agent-instructions:"
    cat .agent-instructions
else
    log_warn "Failed to generate instructions"
    exit 1
fi

# Create initial system prompt if it doesn't exist
if [ ! -f ".h-sophon-agent-init" ]; then
    log_info "Creating initial system prompt..."
    {
        echo "backend: anthropic"
        echo "messages:"
        echo "- role: system"
        echo "  text: |-"
        echo "    You are an expert software development assistant. Your role is to help implement the specified goal"
        echo "    while following best practices and maintaining code quality."
        echo ""
        echo "    Key Capabilities:"
        echo "    1. Code Analysis and Understanding"
        echo "    2. Best Practices Implementation"
        echo "    3. Error Handling and Logging"
        echo "    4. Security Considerations"
        echo "    5. Performance Optimization"
        echo "    6. Testing and Quality Assurance"
        echo ""
        echo "    Guidelines:"
        echo "    1. Follow language-specific best practices"
        echo "    2. Maintain existing code style"
        echo "    3. Include proper error handling"
        echo "    4. Add appropriate logging"
        echo "    5. Consider security implications"
        echo "    6. Think about performance"
        echo "    7. Keep code maintainable"
        echo "    8. Add necessary tests"
        echo ""
        echo "    Always output changes in txtar format and include a summary file."
    } > .h-sophon-agent-init
    log_info "Created initial system prompt"
fi

# Clean up
rm -f .agent-context

log_info "Agent initialization complete"