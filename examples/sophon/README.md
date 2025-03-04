# Sophon Agent

Sophon is an AI agent framework built on top of the cgpt command-line tool, enabling more sophisticated human-AI collaboration for software development tasks.

## Overview

The Sophon agent provides a structured way to leverage LLM capabilities for iterative software development tasks. It uses a cycle-based approach where the AI analyzes context, generates instructions, implements changes, and learns from feedback.

## Key Features

- **Autonomous Development Cycles**: Sophon runs in iterative cycles, implementing requested features or fixes
- **txtar Format**: Uses structured file format for proposing and implementing changes to multiple files
- **Context-Aware**: Understands project structure and history to make informed decisions
- **Self-Improving**: Includes mechanisms for reflection and learning from past attempts

## Components

- **sophon-agent.sh**: Main script for running the agent loop
- **sophon-agent-init.sh**: Script for initializing agent with project context and goals
- **txtar Format**: Standardized format for specifying file changes

## Usage

### Initialize the Agent

```bash
./sophon-agent-init.sh "Implement a feature that reads configuration from a JSON file"
```

This command will:
1. Analyze your project structure
2. Create an `.agent-instructions` file
3. Set up the necessary system prompt

### Run the Agent

```bash
./sophon-agent.sh
```

This will:
1. Start the agent loop
2. Run through implementation cycles
3. Apply changes specified in txtar format
4. Continue until completion or max iterations

## txtar Format Example

The txtar format is used for specifying file changes:

```
-- path/to/file.txt --
Contents of the file go here.
Complete file contents should be provided.

-- another/file.py --
def example():
    return "This is another file"
```

See `txtar-example.txt` for a complete example.

## Advanced Usage

### Configuration Variables

- `HIST_FILE`: History file for the agent (default: `.h-mk`)
- `CYCLE`: Current cycle number (default: 0)
- `ITERATIONS`: Maximum number of iterations (default: 10)
- `SOPHON_PSTART`: System prompt file (default: `.h-sophon-agent-init`)

### Custom Initialization

You can customize the agent initialization by modifying the `.h-sophon-agent-init` file or creating a system prompt template.

## Requirements

- cgpt command-line tool
- yq for YAML processing
- txtar tool for file format handling

## License

Same as the main cgpt project.