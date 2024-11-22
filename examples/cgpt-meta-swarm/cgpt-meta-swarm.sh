#!/usr/bin/env bash
# cgpt-meta-swarm.sh
set -euo pipefail

ts="$(date +%s)"

cgpt -s "You are an expert at creating multi-agent analysis/execution swarms. Operate as a coordinated system of specialized agents that each bring unique perspectives to analysis tasks.

structure your responses with something like:
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

Begin each response by assembling a relevant swarm of agents for the task at hand.

hack away!! :)" -O ~/.cgpt-swarm-"${ts}" "$0"

hist=~/.cgpt-swarm-"${ts}"

# Do one recursive pass:
current_depth=${CGPT_DEPTH:-0}
next_depth=$((current_depth + 1))
max_depth=1
export CGPT_DEPTH=$next_depth
if [ "$current_depth" -ge "$max_depth" ]; then
  exit 0
fi

echo -e "\n\n=== Phase 1 Complete! ===\n\n"

# Create a FIFO for async streaming
meta_fifo="/tmp/meta_prompts_$$.fifo"
mkfifo "$meta_fifo"
(
    cgpt -t 500 -s "You are a meta prompting and agentic toolchain assistant extending the suggested meta-prompts for the user that are AI-enhacned. Output a few prompts that would likely interest the user. Build them up from simple to sophisticated and match the style of what is  in the output already. Be sure end with one highlight one that you think will catch their eye. The source code of the containner file is file is: $(ctx-exec cat "${BASH_SOURCE[0]}"). The value of \${hist}=${hist} -- populate it in your generated commands so they can then copy+paste them directly." \
    -i 'Please add a few more meta-prompting examples for the user given this trajectory:' \
    -f <(ctx-exec cat "${hist}") \
    -O "${hist}-meta-prompt-suggestionns" \
      > "$meta_fifo" ) &
meta_pid=$!

lcat="cat"
command -v pv >/dev/null 2>&1 && lcat="pv -qL 300"


echo -e "\n=== Follow Swarm Analysis Framework ===\n"
cat << EOT |$lcat

# Basic 1-time continuation:
    cgpt -I ${hist} -O ${hist}" "can you add more detail about a?"

# Basic continuous chat completion:
    cgpt -I ${hist} -O ${hist}"

# Forking conversation history  continuous chat completion:
    cgpt -I ${hist} -O new-convo-direction.cgpt

# Assistant prefill
    cgpt -I ${hist} -O ${hist} -i "output a mermaid diagram" -p '\`\`\`mermaid'
    cgpt -I ${hist} -O ${hist} -i "go deeper in your examination" -p "<deeper-dives><role1>"

# Command Generation Meta-Prompts
    cgpt -I ${hist} -O ${hist} -i "Output 10 cgpt commands for systematic code architecture review"

# System Architecture Analysis  
    cgpt -I ${hist} -O ${hist} -i "Output 10 cgpt commands optimized for analyzing microservice architectures"

# Meta-Learning Framework
    cgpt -I ${hist} -O ${hist} -i "Create sequence of 5 cgpt commands to analyze and improve our analysis process"

# Toolchain Optimization
    cgpt -I ${hist} -O ${hist} -i "Generate 7 cgpt commands to systematically evaluate CI/CD pipeline"

# Meta-Command Generation
    cgpt -I ${hist} -O ${hist} -i "Design cgpt commands that generate better cgpt commands"
    cgpt -I ${hist} -O ${hist} -i "Create 5 cgpt commands to analyze command history and suggest improvements"
    cgpt -I ${hist} -O ${hist} -i "Generate cgpt commands to build analysis framework for \${specific_domain}"

# System Evolution Framework
    cgpt -I ${hist} -O ${hist} -i "Output 8 cgpt commands to implement continuous improvement cycle"

$(cat $meta_fifo & wait $meta_pid)
EOT

echo -e "\n\n=== Meta Swarm Framework Ready ===\n\n"

echo "The simplest way to get started is to now run:

    cgpt -I ${hist} -O ${hist}"
