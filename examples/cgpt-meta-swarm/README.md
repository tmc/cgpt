# Meta-Swarm Framework

A sophisticated framework for coordinated multi-agent analysis through cgpt, enabling collaborative AI problem-solving.

## Components

| Script | Description |
|--------|-------------|
| `agent-analysis.sh` | Multi-agent coordination system |
| `perspective-coordinator.sh` | Perspective analysis system |
| `cgpt-meta-swarm.sh` | Core swarm framework with meta-prompting |

## Analysis Patterns

### 1. Agent-based Analysis
- Multiple specialized agents with focused domains of expertise
- Coordinated insights across disciplines
- Structured analysis framework with XML formatting
- Synthesized recommendations with prioritized actions

### 2. Perspective Coordination
- Structured task requirement analysis
- Technical domain identification
- Multi-viewpoint evaluation from different expertise angles
- Conflict resolution between perspectives

### 3. Meta-swarm Operations
- Dynamic agent assembly based on problem domain
- Recursive analysis capabilities
- Self-improvement loops for continuous enhancement
- Pattern recognition across multiple analyses

## Usage

### Basic Analysis

```bash
# Core analysis patterns
./agent-analysis.sh "analyze this codebase"
./perspective-coordinator.sh "evaluate this API design"
./cgpt-meta-swarm.sh "improve these patterns"

# Meta-swarm with options
./cgpt-meta-swarm.sh --depth 2 "recursive analysis"
./cgpt-meta-swarm.sh --no-suggestions "minimal output mode"
```

### Advanced Operations

```bash
# Continue analysis conversation
cgpt -I $HIST -O $HIST

# Add specific perspective
cgpt -I $HIST -O $HIST -p '<agent role="security-expert">'

# Fork analysis into new path
cgpt -I $HIST -O new-direction.cgpt

# Create diagram
cgpt -I $HIST -O $HIST -i "create architecture diagram" -p '```mermaid'
```

### Command Generation

```bash
# Generate meta-commands
cgpt -I $HIST -O $HIST -i "Output 5 cgpt commands for code architecture review"

# Create analysis sequence
cgpt -I $HIST -O $HIST -i "Create sequence of commands to analyze our workflow"
```

See individual scripts for detailed documentation and additional options.

