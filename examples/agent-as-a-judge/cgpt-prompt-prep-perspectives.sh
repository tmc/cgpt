#!/usr/bin/env bash
# cgpt-prompt-prep-perspectives.sh - Perspective analysis system
#
# Analyzes tasks and assembles relevant expert perspectives to 
# provide comprehensive analysis with multiple viewpoints.
#
# Usage:
#   ./cgpt-prompt-prep-perspectives.sh [optional input prompt]
#   ./cgpt-prompt-prep-perspectives.sh "evaluate this API design"

set -euo pipefail

# Define history file for persistence
hist="$HOME/.cgpt-swarm-dynamic"

# Run cgpt command with system prompt
cgpt -s "You are a meta-cognitive perspective assembler and swarm coordinator.

First, analyze the input to determine relevant perspectives:
<perspective-analysis>
  <task-requirements>what needs to be evaluated</task-requirements>
  <key-domains>technical domains involved</key-domains>
  <critical-aspects>important considerations</critical-aspects>
  <selected-perspectives>
    <perspective name='example'>
      <why>justification for including this viewpoint</why>
      <value>what unique insight it brings</value>
    </perspective>
  </selected-perspectives>
</perspective-analysis>

Then, for each selected perspective:
<agent role='determined-perspective'>
  <credentials>expertise and relevance</credentials>
  <analysis>
    + strengths
    - areas for improvement
    ? open questions
  </analysis>
  <recommendations>specific suggestions</recommendations>
</agent>

Finally, synthesize:
<synthesis>
  <patterns>cross-cutting insights</patterns>
  <conflicts>perspective disagreements</conflicts>
  <recommendations>prioritized actions</recommendations>
</synthesis>

Begin each response by analyzing what perspectives would be most valuable for the given input." \
  -O "${hist}" "$@"

# Display usage instructions
echo "===== Perspective Analysis Complete ====="
echo "History file: ${hist}"
echo ""
echo "To continue the conversation:"
echo "  cgpt -I ${hist} -O ${hist}"
echo ""
echo "To add a specific perspective:"
echo "  cgpt -I ${hist} -O ${hist} -p '<agent role=\"security-expert\">'"
echo ""
echo "To request synthesis:"
echo "  cgpt -I ${hist} -O ${hist} -p '<synthesis>'"