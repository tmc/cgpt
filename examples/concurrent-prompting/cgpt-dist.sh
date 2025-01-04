#!/bin/bash

# Create working directory for node outputs
mkdir -p .cgpt-nodes

# Node 1: Architecture Expert
cat > .cgpt-nodes/node1-prompt << 'NODE1'
you are node1 - the architecture expert.
Use these tags to communicate:
<arch-insight>your architectural observations</arch-insight>
<question>what you need to know from other nodes</question>
<build-on>responding to other nodes' insights</build-on>
NODE1

# Node 2: Implementation Expert
cat > .cgpt-nodes/node2-prompt << 'NODE2'
you are node2 - the implementation specialist.
Use these tags to communicate:
<impl-insight>your implementation observations</impl-insight>
<answer>responding to questions</answer>
<suggest>implementation proposals</suggest>
NODE2

# Run the nodes
echo "Analyzing: $1" | cgpt -s "$(cat .cgpt-nodes/node1-prompt)" > .cgpt-nodes/node1-output
echo "Node 1 says: $(cat .cgpt-nodes/node1-output)" | cgpt -s "$(cat .cgpt-nodes/node2-prompt)" > .cgpt-nodes/node2-output

# Sync their insights
cat > .cgpt-nodes/sync-prompt << 'SYNC'
you are the sync coordinator. Analyze and synthesize:
<node1>$(cat .cgpt-nodes/node1-output)</node1>
<node2>$(cat .cgpt-nodes/node2-output)</node2>
Create a unified insight using:
<synthesis>combined insights</synthesis>
<next-steps>recommended actions</next-steps>
SYNC

cgpt -s "$(cat .cgpt-nodes/sync-prompt)" > .cgpt-nodes/sync-output
cat .cgpt-nodes/sync-output
