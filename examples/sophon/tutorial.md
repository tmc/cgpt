# Sophon Agent Tutorial

This tutorial will guide you through creating and using the Sophon agent for AI-assisted software development.

## What is Sophon?

Sophon is an agent framework built on top of the cgpt command-line tool, designed to facilitate iterative software development through AI assistance. It combines:

- A well-structured system prompt
- Iterative development cycles
- The txtar format for file modifications
- Context-aware capabilities

## Prerequisites

To use Sophon, you need:

1. The cgpt command-line tool installed
2. The txtar tool for file format handling
3. yq for YAML processing
4. Basic bash environment

## Setup Process

### 1. Create a Project Directory

Start by creating a directory for your project:

```bash
mkdir my-project
cd my-project
```

### 2. Define Your Requirements

Create a file describing what you want to build:

```bash
# requirements.txt
Create a simple command-line tool that...
```

### 3. Initialize the Sophon Agent

Run the initialization script with your goal:

```bash
../path/to/sophon-agent-init.sh "Implement the requirements in requirements.txt"
```

This will:
- Analyze your project context
- Create an `.agent-instructions` file
- Set up the initial system prompt

### 4. Run the Agent

Start the agent loop:

```bash
../path/to/sophon-agent.sh
```

The agent will:
1. Read the instructions
2. Generate code in txtar format
3. Apply the changes to your project
4. Repeat until completion

## The txtar Format

Sophon uses the txtar format to specify file changes. This format allows for multiple files to be edited in a single response:

```
-- path/to/file.txt --
This is the complete content of file.txt.
The agent always provides the entire file content.

-- another/file.py --
def example():
    return "This is another file"
```

## Advanced Usage

### Custom System Prompts

You can create a custom system prompt in `.h-sophon-agent-init`:

```yaml
backend: anthropic
messages:
- role: system
  text: |-
    You are Sophon, an expert in...
```

### Configuration Variables

Adjust behavior with environment variables:
- `HIST_FILE`: History file (default: `.h-mk`)
- `CYCLE`: Starting cycle number (default: 0)
- `ITERATIONS`: Maximum iterations (default: 10)

## Best Practices

1. **Clear Requirements**: Provide detailed, specific requirements
2. **Initial Structure**: Give the agent some basic structure to work with
3. **Iterative Approach**: Let the agent build up the solution across multiple cycles
4. **Review Each Cycle**: Check the agent's work at each step

## Example Workflow

Here's a typical workflow:

1. Define requirements
2. Initialize the agent
3. Run the first cycle
4. Review the implemented files
5. Refine the instructions if needed
6. Continue the cycle until completion

## Troubleshooting

Common issues:

- **Missing Tools**: Ensure txtar and yq are installed
- **System Prompt Issues**: Check `.h-sophon-agent-init` for proper formatting
- **Context Limitations**: Break complex projects into manageable pieces

## Next Steps

To learn more:
- Explore the example projects in `examples/sophon/`
- Review the source code of the agent scripts
- Try implementing your own projects with Sophon