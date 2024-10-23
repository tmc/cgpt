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

Examples: # Basic query about interpreting command output
$ echo "how should I interpret the output of nvidia-smi?" | cgpt

    # Quick explanation request
    $ echo "explain plan 9 in one sentence" | cgpt

Advanced Examples: # Using a system prompt for a specific assistant role
$ cgpt -s "You are a helpful programming assistant" -i "Write a Python function to calculate the Fibonacci sequence"

    # Code review using input from a file
    $ cat complex_code.py | cgpt -s "You are a code reviewer. Provide constructive feedback." -m "claude-3-5-sonnet-20241022"

    # Interactive session for creative writing
    $ cgpt -c -s "You are a creative writing assistant" # Start an interactive session for story writing

    # Show more advanced examples:
    $ cgpt --show-advanced-usage basic
    $ cgpt --show-advanced-usage all

## Advanced Usage

These examples showcase more sophisticated uses of cgpt, demonstrating its flexibility and power.

### Shell Script Generation

```shell
# Generate a shell script using AI assistance
$ echo "Write a script that analyzes the current local git branch, the recent activity, and suggest a meta-learning for making more effective progress." \
     | cgpt --system-prompt "You are a self-improving unix toolchain assistant. Output a program that uses AI to the goals of the user. The current system is $(uname). The help for cgpt is <cgpt-help-output>$(cgpt --help 2>&1). Your output should be only valid bash. If you have commentary make sure it is prefixed with comment characters." \
     --prefill "#!/usr/bin/env" | tee suggest-process-improvement.sh
```

### Research Analysis

```shell
# Analyze research notes with a high token limit
$ cgpt -f research_notes.txt -s "You are a research assistant. Summarize the key points and suggest follow-up questions." -t 8000
```

### Git Commit Analysis

```shell
# Analyze git commit history
$ git log --oneline | cgpt -s "You are a git commit analyzer. Provide insights on commit patterns and suggest improvements."
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
" -m "claude-3-5-sonnet-20241022" -t 500); echo "Suggested fix:"; echo "$fix"; read -p "Apply this fix? (y/n) " confirm; if [ "$confirm" = "y" ]; then echo "$fix" | bash; sed -i '1d' BUGS.txt; echo "Bug resolved."; else echo "Fix skipped."; fi; echo "Remaining bugs: $(wc -l < BUGS.txt)"; done; echo "All bugs resolved or skipped!"
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

1. Use the `-t` flag to set a higher token limit for complex tasks or longer outputs.
2. Combine cgpt with other command-line tools using pipes for powerful workflows.
3. Save frequently used system prompts in a configuration file for quick access.
4. Use the `--debug` flag to see detailed information about the API requests and responses.
5. Experiment with different models using the `-m` flag to find the best fit for your task.

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
