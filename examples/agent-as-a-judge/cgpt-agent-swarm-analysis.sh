#!/usr/bin/env bash
# cgpt-agent-swarm-analysis.sh - Multi-agent analysis coordinator
#
# Creates a swarm of specialized AI agents that analyze a problem from
# multiple perspectives and synthesize insights.
#
# Usage: 
#   ./cgpt-agent-swarm-analysis.sh [optional prompt]
#   ./cgpt-agent-swarm-analysis.sh "analyze this system architecture"

set -euo pipefail

# Generate timestamp for unique history file
ts="$(date +%s)"
hist="$HOME/.cgpt-swarm-${ts}"

# Run initial cgpt command with system prompt
cgpt -s "You are an expert at creating multi-agent analysis swarms. Operate as a coordinated system of specialized agents that each bring unique perspectives to analysis tasks.

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

Begin each response by assembling a relevant swarm of agents for the task at hand." \
  -O "${hist}" "$@"

# Display continuation instructions
echo "===== Agent Swarm Analysis Complete ====="
echo "History file: ${hist}"
echo ""
echo "To continue the conversation:"
echo "  cgpt -I ${hist} -O ${hist}"
echo ""
echo "For creative prefills (directing the AI response):"
echo "  cgpt -I ${hist} -O ${hist} -p '<agent role=\"security\">'"
echo "  cgpt -I ${hist} -O ${hist} -p '<synthesis><key-findings>'"
echo ""
echo "To fork the conversation into a new file:"
echo "  cgpt -I ${hist} -O new-analysis.cgpt"

