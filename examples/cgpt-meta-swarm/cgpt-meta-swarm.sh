#!/usr/bin/env bash
# cgpt-meta-swarm.sh - Meta-level swarm coordination framework
#
# This script implements a sophisticated multi-agent coordination system
# that enables recursive analysis and self-improvement through agent swarms.
#
# Features:
# - Dynamic agent assembly
# - Perspective coordination
# - Meta-analysis capabilities
# - Self-improvement loops
# - Command generation suggestions
#
# Usage:
#   ./cgpt-meta-swarm.sh [optional focus area]
#
# Example:
#   ./cgpt-meta-swarm.sh "analyze this codebase"
#   ./cgpt-meta-swarm.sh "improve these prompts"
#   ./cgpt-meta-swarm.sh --no-suggestions "minimal output mode"

set -euo pipefail

# Configuration
MAX_DEPTH=${MAX_DEPTH:-1}        # Maximum recursion depth
SHOW_SUGGESTIONS=${SHOW_SUGGESTIONS:-true}  # Show command suggestions
MAX_TOKENS=${MAX_TOKENS:-500}    # Token limit for meta-prompting

# Parse command line arguments
POSITIONAL_ARGS=()
while [[ $# -gt 0 ]]; do
  case $1 in
    --no-suggestions)
      SHOW_SUGGESTIONS=false
      shift
      ;;
    --depth)
      MAX_DEPTH="$2"
      shift 2
      ;;
    --tokens)
      MAX_TOKENS="$2"
      shift 2
      ;;
    --help|-h)
      echo "Usage: $0 [options] [prompt]"
      echo ""
      echo "Options:"
      echo "  --no-suggestions    Skip command suggestion generation"
      echo "  --depth NUMBER      Set maximum recursion depth (default: 1)"
      echo "  --tokens NUMBER     Token limit for meta-prompting (default: 500)"
      echo "  --help, -h          Show this help message"
      echo ""
      echo "Examples:"
      echo "  $0 \"analyze this system architecture\""
      echo "  $0 --depth 2 \"recursive analysis of codebase\""
      echo "  $0 --no-suggestions \"minimal output\""
      exit 0
      ;;
    *)
      POSITIONAL_ARGS+=("$1")
      shift
      ;;
  esac
done
set -- "${POSITIONAL_ARGS[@]}" # restore positional parameters

# Generate timestamp for unique history file
ts="$(date +%s)"
hist="$HOME/.cgpt-swarm-${ts}"

# Define color codes for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Helper function for formatted output
print_section() {
  echo -e "\n${BLUE}=== $1 ===${NC}\n"
}

# Run initial cgpt command with system prompt
print_section "Initializing Meta-Swarm Framework"

cgpt -s "You are an expert at creating multi-agent analysis/execution swarms. Operate as a coordinated system of specialized agents that each bring unique perspectives to analysis tasks.

Structure your responses with:
<agent-swarm>
  <agent role='perspective-name'>
    <credentials>why this agent is qualified</credentials>
    <analysis>key insights from this perspective</analysis>
    <recommendations>specific suggestions</recommendations>
  </agent>

  <!-- common roles include but are not limited to: -->
  <!-- - domain experts (go, unix, testing, etc) -->
  <!-- - meta-process experts (design, architecture) -->
  <!-- - critical perspective (edge cases, failures) -->
  <!-- - synthesis (combining insights) -->

  <synthesis>
    <key-findings>main insights across agents</key-findings>
    <action-items>concrete next steps</action-items>
  </synthesis>
</agent-swarm>

Use clear XML structure to show how different perspectives analyze the problem. Each agent should:
1. Stay focused on their expertise
2. Provide concrete examples
3. Note both positives and negatives
4. Make specific recommendations

The synthesis agent should:
1. Find patterns across perspectives
2. Resolve any conflicts
3. Prioritize recommendations
4. Suggest concrete next steps

Note that this tool is cgpt, a Unix-friendly command line AI tool.

<cgpt-usage>$(cgpt -h 2>&1)</cgpt-usage>

Begin each response by assembling a relevant swarm of agents for the task at hand." \
  -O "${hist}" "$@"

# Handle recursive analysis if requested
current_depth=${CGPT_DEPTH:-0}
next_depth=$((current_depth + 1))
export CGPT_DEPTH=$next_depth

if [ "$current_depth" -ge "$MAX_DEPTH" ]; then
  echo -e "${YELLOW}Maximum recursion depth reached (${MAX_DEPTH}).${NC}"
else
  print_section "Phase 1 Complete"
  echo -e "${GREEN}Initial analysis complete. Moving to meta-analysis phase...${NC}"
fi

# Skip suggestions if requested
if [ "$SHOW_SUGGESTIONS" = false ]; then
  print_section "Command Suggestions Skipped"
  echo -e "${CYAN}Command suggestions disabled. Use --suggestions to enable.${NC}"
else
  # Generate command suggestions asynchronously
  print_section "Generating Command Suggestions"
  
  # Create a FIFO for async streaming
  meta_fifo="/tmp/meta_prompts_$$.fifo"
  mkfifo "$meta_fifo"
  
  # Run the suggestion generation in background
  (
    cgpt -t "$MAX_TOKENS" -s "You are a meta prompting and agentic toolchain assistant extending the suggested meta-prompts for the user that are AI-enhanced. Output a few prompts that would likely interest the user. Build them up from simple to sophisticated and match the style of what is in the output already. End with one highlight that you think will catch their eye. The value of \${hist}=${hist} -- populate it in your generated commands so they can be directly copy+pasted." \
      -i 'Please add a few more meta-prompting examples for the user:' \
      -f <(cat "${hist}") \
      -O "${hist}-meta-prompt-suggestions" \
      > "$meta_fifo"
  ) &
  meta_pid=$!
  
  # Check if pv is available for smoother output
  lcat="cat"
  command -v pv >/dev/null 2>&1 && lcat="pv -qL 300"
  
  # Display command templates and suggestions
  echo -e "\n${CYAN}=== Swarm Analysis Framework Command Templates ===${NC}\n"
  cat << EOT |$lcat
# Basic Continuation
cgpt -I ${hist} -O ${hist}

# Ask Follow-up Question
cgpt -I ${hist} -O ${hist} -i "Can you add more detail about X?"

# Fork Conversation
cgpt -I ${hist} -O new-analysis.cgpt

# Prefill Responses (Direct AI Output)
cgpt -I ${hist} -O ${hist} -p '<agent role="security-expert">'
cgpt -I ${hist} -O ${hist} -i "create a diagram" -p '\`\`\`mermaid'
cgpt -I ${hist} -O ${hist} -i "dive deeper" -p "<deeper-analysis>"

# Generate Meta-Commands
cgpt -I ${hist} -O ${hist} -i "Output 5 cgpt commands for code architecture review"
cgpt -I ${hist} -O ${hist} -i "Create sequence of commands to analyze our workflow"
cgpt -I ${hist} -O ${hist} -i "Design cgpt commands that generate better cgpt commands"

# Advanced Analysis Patterns
cgpt -I ${hist} -O ${hist} -i "Generate commands to build analysis framework for X"
cgpt -I ${hist} -O ${hist} -i "Output commands to implement continuous improvement cycle"

# Custom Suggestions
$(cat "$meta_fifo" & wait "$meta_pid")
EOT
fi

print_section "Meta-Swarm Framework Ready"

echo -e "${GREEN}Your meta-swarm analysis is complete!${NC}"
echo -e "${CYAN}History file: ${YELLOW}${hist}${NC}"
echo ""
echo -e "The simplest way to continue is to run:"
echo -e "    ${YELLOW}cgpt -I ${hist} -O ${hist}${NC}"
echo ""
echo -e "For help with more advanced commands, run:"
echo -e "    ${YELLOW}$0 --help${NC}"

# Clean up
if [ -f "$meta_fifo" ]; then
  rm -f "$meta_fifo"
fi
