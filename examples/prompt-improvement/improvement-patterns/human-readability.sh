#!/usr/bin/env bash
# human-readability.sh - Pattern for improving human readability of AI outputs
#
# This script provides patterns for improving the readability and
# scannability of AI outputs while maintaining machine parseability.
#
# Features:
# - Visual hierarchy
# - Information chunking
# - Progress indicators
# - Decision trees
#
# Usage:
#   ./human-readability.sh <input-file> <output-file>
set -euo pipefail

# Base prompt for readability improvements
READABILITY_PROMPT='You are a readability and information design expert. Your goal is to make AI outputs more scannable and readable for humans while maintaining machine parseability.

KEY PRINCIPLES:
1. Visual Hierarchy
   - Use clear section headers
   - Apply consistent indentation
   - Add visual breaks between sections
   - Highlight key information

2. Information Chunking
   - Group related information
   - Use numbered lists for sequences
   - Limit paragraph length
   - Add summary boxes

3. Formatting Patterns
   - Use emoji markers strategically: 🔍 (analysis), ⚠️ (warnings), ✅ (success), 💡 (insights)
   - Apply consistent spacing
   - Utilize ASCII frames for important sections
   - Add progress indicators

4. Quick-Scan Elements
   - TL;DR sections
   - Key takeaways boxes
   - Decision trees
   - Status indicators

OUTPUT STRUCTURE:
┌─ Summary ───────────────────────┐
│ Quick overview of key points    │
└─────────────────────────────────┘

🔍 Analysis
  • Main point 1
  • Main point 2

💡 Insights
  1. Key insight
  2. Important finding

⚠️ Considerations
  - Watch out for...
  - Be careful with...

✅ Next Steps
  1. First action
  2. Second action

└─ End of Section ─────────────────┘

GUIDELINES:
- Maintain XML structure for machine parsing
- Add human-friendly formatting within XML
- Use consistent visual patterns
- Include progress indicators
- Add status markers'

# Function to add readability improvements
add_readability() {
    local input_file="$1"
    local output_file="$2"
    
    cgpt -s "${READABILITY_PROMPT}" \
        -i "Improve the readability of this output while maintaining its structure:" \
         -f "${input_file}" \
         -O "${output_file}" \
         -p '┌─ Improved Output ─────────────────┐'
}

# Function to add progress indicators
add_progress_markers() {
    local input_file="$1"
    local output_file="$2"
    
    cgpt -s "You are a progress visualization expert. Add clear progress indicators to this output:
- Use [1/5] style markers for steps
- Add completion indicators
- Show status (🟢 Done, 🟡 In Progress, ⚪ Pending)
- Include time estimates where relevant" \
         -f "${input_file}" \
         -O "${output_file}" \
         -p '┌─ Progress Enhanced Output ────────┐'
}

# Function to add decision trees
add_decision_trees() {
    local input_file="$1"
    local output_file="$2"
    
    cgpt -s "You are a decision tree visualization expert. Add ASCII decision trees to this output:
Example:
Should you add a decision tree?
├─ Yes ─┬─ Simple decision → ASCII tree
│       └─ Complex decision → Mermaid diagram
└─ No ──┬─ Linear process → Use steps
        └─ Single option → Use bullet" \
         -f "${input_file}" \
         -O "${output_file}" \
         -p '┌─ Decision Enhanced Output ────────┐'
}

