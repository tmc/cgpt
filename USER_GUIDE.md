# cgpt User Guide

cgpt is a powerful command-line tool for interacting with Large Language Models (LLMs) from multiple providers. This comprehensive guide will help you get started and make the most of cgpt's features.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Quick Start](#quick-start)
3. [Common Use Cases](#common-use-cases)
4. [Configuration Guide](#configuration-guide)
5. [Advanced Features](#advanced-features)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)
8. [Tips and Tricks](#tips-and-tricks)

## Getting Started

### Prerequisites

Before using cgpt, you'll need:

- **Go 1.23 or higher** (if building from source)
- **API key** from at least one supported provider:
  - Anthropic API key (recommended default)
  - OpenAI API key
  - Google AI API key
  - OpenRouter API key (for accessing multiple models)

### Installation

Choose one of the following installation methods:

#### Option 1: Homebrew (macOS/Linux - Recommended)
```bash
brew install tmc/tap/cgpt
```

#### Option 2: Go Install
```bash
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

#### Option 3: Download Binary
Download the latest release from [GitHub Releases](https://github.com/tmc/cgpt/releases).

### First-Time Setup

1. **Set your API key** (choose one):
   ```bash
   # For Anthropic (recommended default)
   export ANTHROPIC_API_KEY='your-anthropic-api-key-here'

   # For OpenAI
   export OPENAI_API_KEY='your-openai-api-key-here'

   # For Google AI
   export GOOGLE_API_KEY='your-google-api-key-here'

   # For OpenRouter (access to multiple models)
   export OPENROUTER_API_KEY='your-openrouter-api-key-here'
   ```

2. **Add to your shell profile** for persistence:
   ```bash
   echo 'export ANTHROPIC_API_KEY="your-key-here"' >> ~/.bashrc
   source ~/.bashrc
   ```

3. **Test your installation**:
   ```bash
   cgpt --version
   echo "Hello, cgpt!" | cgpt
   ```

## Quick Start

### Basic Usage

```bash
# Simple query
echo "Explain quantum computing in simple terms" | cgpt

# Direct input
cgpt -i "Write a Python function to calculate factorial"

# Interactive mode
cgpt -c

# Using a different model
cgpt -m gpt-4 -i "Explain the theory of relativity"

# With system prompt
cgpt -s "You are a helpful coding assistant" -i "Debug this Python code"
```

### Getting Help

```bash
# Show basic help
cgpt --help

# Show advanced usage examples
cgpt --show-advanced-usage all

# Show specific sections
cgpt --show-advanced-usage basic,tips
```

## Common Use Cases

### 1. Code Generation and Review

#### Generate Code
```bash
cgpt -s "You are an expert programmer" -i "Write a REST API in Go with user authentication"
```

#### Code Review
```bash
cat my_code.py | cgpt -s "You are a senior developer. Review this code and suggest improvements."
```

#### Debug Issues
```bash
cgpt -s "You are a debugging expert" -i "This Python code throws a KeyError. How do I fix it?" -f error_log.txt
```

### 2. Writing and Documentation

#### Generate Documentation
```bash
cgpt -s "You are a technical writer" -i "Create API documentation for this Go package" -f main.go
```

#### Writing Assistance
```bash
cgpt -c -s "You are a professional writer. Help me improve my writing style and clarity."
```

### 3. Data Analysis and Research

#### Analyze Data
```bash
cat data.csv | cgpt -s "You are a data analyst. Analyze this CSV data and provide insights." -t 8000
```

#### Research Summary
```bash
cgpt -f research_paper.txt -s "Summarize the key findings and methodology of this research paper."
```

### 4. System Administration

#### Log Analysis
```bash
tail -n 100 /var/log/syslog | cgpt -s "You are a sysadmin. Analyze these logs for potential issues."
```

#### Script Generation
```bash
cgpt -s "You are a shell scripting expert" -i "Create a backup script for MySQL databases"
```

### 5. Learning and Education

#### Concept Explanation
```bash
cgpt -s "You are a patient teacher" -i "Explain machine learning concepts using simple analogies"
```

#### Practice Problems
```bash
cgpt -s "You are a computer science instructor" -i "Generate 5 algorithm practice problems with solutions"
```

## Configuration Guide

### Configuration File

Create a `config.yaml` file in your current directory or `~/.cgpt/config.yaml`:

```yaml
# Basic configuration
backend: "anthropic"
model: "claude-sonnet-4-20250514"
stream: true
maxTokens: 8000
temperature: 0.05

# System prompt (optional)
systemPrompt: "You are a helpful AI assistant."

# Advanced features
promptCaching: true
showUsage: true
debug: false

# API keys (optional - can use environment variables instead)
anthropicAPIKey: "your-key-here"
openaiAPIKey: "your-key-here"
googleAPIKey: "your-key-here"
openrouterAPIKey: "your-key-here"

# Reasoning/thinking features (for compatible models)
thinkingMode: "medium"
thinkingBudget: 2048
showReasoning: true
interleavedThinking: false
```

### Environment Variables

All configuration options can be set via environment variables with the `CGPT_` prefix:

```bash
export CGPT_BACKEND="anthropic"
export CGPT_MODEL="claude-sonnet-4-20250514"
export CGPT_MAX_TOKENS=8000
export CGPT_TEMPERATURE=0.05
export CGPT_SYSTEM_PROMPT="You are a helpful assistant"
export CGPT_PROMPT_CACHING=true
export CGPT_SHOW_USAGE=true
```

### Backend Selection

cgpt automatically detects the appropriate backend based on the model name:

```bash
# Auto-detected as Anthropic
cgpt -m claude-sonnet-4

# Auto-detected as OpenAI
cgpt -m gpt-4

# Auto-detected as Google AI
cgpt -m gemini-pro

# Auto-detected as OpenRouter (provider/model format)
cgpt -m anthropic/claude-sonnet-4
```

## Advanced Features

### History Management

cgpt provides sophisticated session history management:

#### Auto-generated Sessions
```bash
# Creates timestamped session files in ~/.cgpt/history/sessions/
cgpt -H auto -i "Start a new coding project discussion"
```

#### Continue Previous Session
```bash
# Continue your most recent session
cgpt -C -i "What were we discussing?"
```

#### Manual History Control
```bash
# Read from one file, write to another (fork)
cgpt -I old_session.yaml -O new_session.yaml -i "Let's try a different approach"

# Use same file for input and output
cgpt -H my_session.yaml -i "Continue our conversation"
```

### Reasoning and Thinking Features

For models that support reasoning (Anthropic Claude 4+, OpenAI o1+, Google Gemini 2.5+):

```bash
# Enable thinking mode
cgpt --thinking-mode medium -i "Solve this complex math problem step by step"

# Set explicit thinking budget
cgpt --thinking-budget 2048 -i "Analyze this complex system architecture"

# Show reasoning process
cgpt --show-reasoning -i "Debug this algorithm"

# Interleaved thinking (Claude 4+ only)
cgpt --interleaved-thinking -i "Design a complex software system"
```

### Prompt Caching

Reduce costs for repeated prompts:

```bash
# Enable prompt caching
cgpt --prompt-caching -s "You are a code reviewer with these standards..." -f code1.py
cgpt --prompt-caching -s "You are a code reviewer with these standards..." -f code2.py
```

### Usage Tracking

Monitor token usage and costs:

```bash
# Show detailed usage information
cgpt --usage -i "Analyze this data"
```

## Best Practices

### 1. System Prompts

- **Be specific**: Instead of "You are helpful", use "You are a Python expert specializing in data analysis"
- **Set context**: Include relevant background information
- **Define output format**: Specify desired response structure
- **Use examples**: Show the AI what good output looks like

```bash
cgpt -s "You are a senior software engineer with 10 years of experience in Go development.
When reviewing code, focus on:
1. Performance implications
2. Error handling
3. Code readability
4. Best practices
Format your response as: Issue -> Explanation -> Suggested Fix" -f code.go
```

### 2. Model Selection

- **Claude Sonnet 4**: Best for complex reasoning and code analysis
- **GPT-4**: Good for creative tasks and general conversation
- **Gemini Pro**: Fast and efficient for straightforward tasks
- **OpenRouter**: Access to specialized models

### 3. Token Management

- Use `-t` flag to set appropriate token limits
- Enable `--usage` to monitor token consumption
- Use prompt caching for repeated system prompts

### 4. Interactive Sessions

- Use `-c` for exploratory conversations
- Save important sessions with `-H`
- Use `-C` to continue previous discussions

### 5. File Processing

- Process large files in chunks
- Use appropriate system prompts for file types
- Combine with shell tools for preprocessing

## Troubleshooting

### Common Issues

#### 1. API Key Problems

**Error**: `API key not found` or authentication failures

**Solutions**:
- Verify environment variable is set: `echo $ANTHROPIC_API_KEY`
- Check for typos in API key
- Ensure key has necessary permissions
- Try setting key in config file instead

```bash
# Debug API key issues
cgpt --verbose -i "test" 2>&1 | grep -i "key\|auth"
```

#### 2. Network Issues

**Error**: `Request timed out` or connection failures

**Solutions**:
- Check internet connectivity
- Increase timeout: `--completion-timeout 5m`
- Try different backend
- Check firewall/proxy settings

#### 3. Model Not Found

**Error**: `Model not found` or unsupported model

**Solutions**:
- Check model name spelling
- Verify model is available for your API key
- Use `cgpt --help` to see supported models
- Try alternative model: `-m claude-3-5-sonnet`

#### 4. Output Issues

**Error**: Truncated or unexpected output

**Solutions**:
- Increase token limit: `-t 16000`
- Adjust temperature: `-T 0.1` for more focused output
- Refine system prompt
- Try different model

#### 5. Configuration Problems

**Error**: Config file not found or invalid

**Solutions**:
- Check config file location and permissions
- Validate YAML syntax
- Use `--config` to specify custom location
- Enable verbose mode for debugging

### Debug Mode

Enable debug mode for detailed troubleshooting:

```bash
cgpt --debug -i "test query" 2>&1 | less
```

This shows:
- HTTP requests/responses
- Configuration loading
- Model initialization
- Token usage details

### Getting Help

1. **Check documentation**: This guide and `cgpt --help`
2. **Enable verbose mode**: `cgpt --verbose` for detailed output
3. **Use debug mode**: `cgpt --debug` for troubleshooting
4. **Check GitHub issues**: Search existing issues and solutions
5. **Create bug report**: Include debug output and configuration

## Tips and Tricks

### 1. Shell Integration

#### Create Aliases
```bash
# Add to ~/.bashrc or ~/.zshrc
alias cgpt-code='cgpt -s "You are an expert programmer" -t 8000'
alias cgpt-explain='cgpt -s "You are a patient teacher. Explain concepts clearly"'
alias cgpt-debug='cgpt -s "You are a debugging expert. Provide step-by-step solutions"'
```

#### Pipeline Processing
```bash
# Process multiple files
find . -name "*.py" -exec cat {} \; | cgpt -s "Review this Python codebase"

# Combine with other tools
git log --oneline -n 10 | cgpt -s "Analyze these commit messages for patterns"

# Use with curl for web content
curl -s https://api.github.com/repos/user/repo | cgpt -s "Summarize this GitHub repository"
```

### 2. Advanced Workflows

#### Iterative Development
```bash
# Start a persistent session for a project
cgpt -H project_session.yaml -c -s "You are my coding assistant for a web application project"
```

#### Automated Processing
```bash
#!/bin/bash
# Script to process documentation files
for file in docs/*.md; do
    echo "Processing $file..."
    cgpt -s "Improve this documentation for clarity and completeness" -f "$file" > "${file}.improved"
done
```

#### Multi-step Analysis
```bash
# Step 1: Generate summary
cgpt -f data.txt -s "Create a brief summary" -O summary.yaml

# Step 2: Generate insights based on summary
cgpt -I summary.yaml -s "Based on the summary, provide actionable insights" -O insights.yaml

# Step 3: Create report
cgpt -I insights.yaml -s "Create a professional report" > final_report.md
```

### 3. Model-Specific Tips

#### Anthropic Claude
- Excellent for complex reasoning and code analysis
- Use thinking modes for step-by-step problem solving
- Enable prompt caching for repeated system prompts
- Good at following detailed instructions

#### OpenAI GPT
- Great for creative writing and brainstorming
- Use o1 models for complex reasoning tasks
- Works well with conversational interfaces
- Good at maintaining context in long conversations

#### Google Gemini
- Fast and efficient for straightforward tasks
- Good at processing structured data
- Excellent for summarization and analysis
- Cost-effective for high-volume usage

### 4. Configuration Tips

#### Development vs Production
```yaml
# Development config - more verbose
backend: "anthropic"
model: "claude-3-5-sonnet"
debug: true
showUsage: true
maxTokens: 16000

# Production config - optimized for cost
backend: "googleai"
model: "gemini-pro"
debug: false
maxTokens: 4000
promptCaching: true
```

#### Task-Specific Configs
```bash
# Create different config files
cp config.yaml coding-config.yaml
cp config.yaml writing-config.yaml

# Use specific config for tasks
cgpt --config coding-config.yaml -i "Review this code"
cgpt --config writing-config.yaml -i "Edit this document"
```

This user guide should help you get started with cgpt and make the most of its powerful features. For the latest updates and additional examples, check the [GitHub repository](https://github.com/tmc/cgpt) and community discussions.