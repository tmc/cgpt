# cgpt Usage Examples

This document provides examples and guidance for using cgpt, a command-line tool for interacting with Large Language Models (LLMs). These examples are compatible with cgpt version 1.0.0 and above. If you're using an older version, please update to the latest release.

## Table of Contents

1. [Basic Usage](#basic-usage)
2. [Advanced Usage](#advanced-usage)
3. [Meta-Prompting](#meta-prompting)
4. [Code Improvements](#code-improvements)
5. [Tips and Tricks](#tips-and-tricks)
6. [Troubleshooting](#troubleshooting)

## Basic Usage

### Simple Queries
```bash
# Ask a basic question
echo "What is the difference between HTTP and HTTPS?" | cgpt

# Get quick explanations
echo "explain quantum computing in one sentence" | cgpt

# Interpret command output
nvidia-smi | cgpt -s "Explain this GPU status output in simple terms"
```

### Interactive Mode
```bash
# Start a chat session
cgpt -c

# Interactive session with a specific role
cgpt -c -s "You are a helpful programming assistant"

# Continue your last conversation
cgpt -C
```

### File Processing
```bash
# Analyze a single file
cgpt -f script.py -s "Review this code for potential bugs"

# Process multiple files
cgpt -f config.yaml -f deploy.sh -i "Help me understand these deployment files"

# Use stdin for file input
cat large_log.txt | cgpt -s "Summarize the key events in this log file"
```

## Advanced Usage

These examples showcase more sophisticated uses of cgpt, demonstrating its flexibility and power.

### Session Management & History
```bash
# Auto-save conversations with timestamps
cgpt -H auto -c

# Save conversation to specific file
cgpt -H my-research-session.json -c

# Load previous conversation and continue
cgpt -I previous-session.json -O updated-session.json -i "Let's continue our discussion"

# Continue most recent session
cgpt -C
```

### AI Model Configuration
```bash
# Use different models for different tasks
cgpt -m claude-haiku-3-20240307 -i "Quick summary of this file" -f report.md  # Fast, cheap
cgpt -m claude-sonnet-4-20250514 -i "Detailed analysis of this code" -f complex.py  # Powerful

# Adjust creativity/randomness
cgpt -T 0.1 -i "Write formal documentation"  # More focused
cgpt -T 0.8 -i "Write a creative story"     # More creative

# Set token limits
cgpt -t 500 -i "Brief explanation of AI"    # Short response
cgpt -t 4000 -i "Comprehensive guide to Go" # Longer response
```

### Advanced AI Features (Claude 4+ Models)
```bash
# Enable reasoning mode for complex problems
cgpt --thinking-mode high -i "Solve this logic puzzle: If all bloops are razzles..."

# Set specific reasoning budget
cgpt --thinking-budget 1000 -i "Plan a complete software architecture"

# Show AI's reasoning process
cgpt --show-reasoning --thinking-mode medium -i "Debug this algorithm"

# Enable prompt caching for repeated queries
cgpt --prompt-caching -f large-codebase.py -i "Find all the functions"
```

### System Integration & Automation
```bash
# Generate shell scripts with context
echo "Write a script that analyzes git branch activity and suggests improvements" | \
  cgpt -s "You are a DevOps expert. Output only valid bash. Use comments for explanations. Context: $(uname -a)" \
  --prefill "#!/bin/bash" | tee analyze-git.sh

# Process command output intelligently
docker ps | cgpt -s "Analyze these running containers and suggest optimizations"

# Batch process multiple files
for file in *.log; do
  echo "=== Processing $file ==="
  cgpt -f "$file" -s "Summarize key events and errors" --usage
done

# Chain with other tools
curl -s https://api.github.com/repos/golang/go/issues | \
  jq '.[] | .title' | \
  cgpt -s "Analyze these GitHub issues and identify common themes"
```

### Code Analysis & Development
```bash
# Multi-file code review
cgpt -f main.go -f config.go -f tests.go \
     -s "You are a senior Go developer. Review this code for: 1) bugs 2) performance 3) best practices"

# Generate tests with context
cgpt -f calculator.py -s "Generate comprehensive unit tests for this Python module" \
     --prefill "import unittest" --max-tokens 2000

# Debug with full context
cgpt -f error.log -f source.go -i "The error log shows failures. Help me debug the source code."
```

## Meta-Prompting

Meta-prompting involves using cgpt to generate prompts or enhance existing ones. This section demonstrates advanced techniques for creating and refining prompts.

### Generating Meta-Prompts

```shell
# Generate a meta-prompt for creating cgpt usage examples
$ echo "Create a meta-prompt that generates prompts for new cgpt usage examples" | cgpt -s "You are an expert in meta-programming and prompt engineering. Your task is to create a meta-prompt that, when used with cgpt, will generate prompts for creating new, innovative cgpt usage examples. The meta-prompt should:

1. Encourage creativity and practical applications
2. Incorporate the style and structure of existing cgpt examples
3. Utilize cgpt's features and options effectively
4. Include the <cgpt-help-output> technique for context
5. Be concise yet comprehensive

Output the meta-prompt as a single-line cgpt command, prefixed with a # comment explaining its purpose. The command should use appropriate cgpt options and should be designed to output a prompt that can be directly used to generate new usage examples.

Here's the cgpt help output for reference:
<cgpt-help-output>
$(cgpt --help 2>&1)
</cgpt-help-output>

Ensure the meta-prompt propagates these techniques forward."
```

### Automatic Prompt Generation

```shell
# Automatically generate a fitting prompt based on user input and wrap it in XML tags
$ echo "Your input text here" | cgpt -s "You are an expert prompt engineer with deep understanding of language models. Your task is to analyze the given input and automatically generate a fitting prompt that would likely produce that input if given to an AI assistant. Consider the following in your analysis:

1. The subject matter and domain of the input
2. The style, tone, and complexity of the language
3. Any specific instructions or constraints implied by the content
4. The likely intent or goal behind the input

Based on your analysis, create a prompt that would guide an AI to produce similar output. Your response should be in this format:

<inferred-prompt>
[Your generated prompt here]
</inferred-prompt>

Explanation: [Brief explanation of your reasoning]

Ensure that the generated prompt is entirely contained within the <inferred-prompt> tags, with no other content inside these tags.

Here's the help output for cgpt for reference:
<cgpt-help-output>$(cgpt --help 2>&1)</cgpt-help-output>

Analyze the following input and generate a fitting prompt:"
```

## Code Improvements

This section demonstrates how to use cgpt for various code improvement tasks.

### Iterative Bug Resolution

```shell
# Iteratively resolve bugs using cgpt until BUGS file is empty, with user confirmation and bash prefill
$ while [ -s BUGS.txt ]; do bug=$(head -n 1 BUGS.txt); echo "Resolving: $bug"; fix=$(echo "Suggest a fix for this bug: $bug" | cgpt -s "You are an expert programmer and debugger. Analyze the given bug and suggest a concise, practical fix. Output only valid bash code or commands needed to resolve the issue." --prefill "#!/bin/bash
# Fix for bug: $bug
" -m "claude-3-7-sonnet-20250219" -t 500); echo "Suggested fix:"; echo "$fix"; read -p "Apply this fix? (y/n) " confirm; if [ "$confirm" = "y" ]; then echo "$fix" | bash; sed -i '1d' BUGS.txt; echo "Bug resolved."; else echo "Fix skipped."; fi; echo "Remaining bugs: $(wc -l < BUGS.txt)"; done; echo "All bugs resolved or skipped!"
```

### Shell Script Debugging

```shell
# General shell script debugger using clipboard and current directory context
$ echo "Debug the following shell script issue: $(pbpaste)" | cgpt -s "You are an expert shell script debugger. Analyze the given issue, using the clipboard content and files in the current directory as context. Suggest explanations and fixes. Your output should be valid bash, including comments for explanations and executable code for fixes." --prefill "#!/bin/bash
# Debugging report and suggested fixes
# Context from current directory:
$(ls -la)
$(head -n 20 *)
# Analysis and suggestions:
"
```

## Tips and Tricks

### Performance & Cost Optimization
- Use `-m claude-haiku-3-20240307` for quick, simple tasks to save on costs
- Use `-m claude-sonnet-4-20250514` for complex reasoning and code analysis
- Enable `--prompt-caching` when making multiple requests with similar context
- Set appropriate `-t` (max-tokens) limits to avoid unnecessarily long responses
- Use `--usage` flag to monitor token consumption and costs

### Input Management
- Combine multiple input methods: `cgpt -i "Context: " -f file1.txt -f file2.txt -i "Question: ..."`
- Use `-f -` to read from stdin when piping: `command | cgpt -f - -s "analyze this"`
- Save complex system prompts in files: `cgpt -f system-prompt.txt -i "Your question"`

### Session & History Management
- Use `-H auto` for automatic timestamped session files
- Use `-C` to quickly continue your last conversation
- Save important conversations with meaningful names: `-H "project-review-2024.json"`
- Use history for context: load previous sessions with `-I session.json`

### Advanced Usage Patterns
- **Chain operations**: `cgpt -i "step 1" | cgpt -s "refine this" | cgpt -s "final polish"`
- **Batch processing**: Use shell loops with cgpt for multiple files
- **Template responses**: Use `--prefill` to guide output format
- **Debug mode**: Use `--debug` to see API requests and troubleshoot issues

### Model-Specific Features
- **Claude 4+ only**: `--thinking-mode`, `--show-reasoning`, `--interleaved-thinking`
- **All models**: Adjust `--temperature` (0.0-1.0) for focus vs creativity
- **OpenAI specific**: Use `--openai-use-max-tokens` if needed for compatibility

### Integration with Other Tools
```bash
# Git workflow integration
git diff | cgpt -s "Review these changes and suggest improvements"

# Log analysis
tail -f app.log | cgpt -s "Monitor this log and alert me to issues"

# Documentation generation
find . -name "*.go" | head -5 | xargs cgpt -f -s "Generate API documentation"

# System administration
ps aux | cgpt -s "Analyze running processes and suggest optimizations"
```

## Troubleshooting

### Common Issues and Solutions

1. **Error: API key not found**

   - Ensure you've set the `CGPT_API_KEY` environment variable with your API key.

2. **Error: Request timed out**

   - Check your internet connection and try again.
   - If the issue persists, try increasing the timeout using the `--completion-timeout` flag.

3. **Output is truncated**

   - Increase the token limit using the `-t` flag.

4. **Unexpected or irrelevant responses**
   - Refine your system prompt to provide more context or constraints.
   - Try using a different model with the `-m` flag.

If you encounter any other issues, please check the official documentation or open an issue on the cgpt GitHub repository.
