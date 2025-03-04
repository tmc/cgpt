# Sophon Agent Quick Start Guide

This guide will help you quickly get started with the Sophon agent for AI-assisted software development.

## Setup

1. Ensure you have the cgpt command-line tool installed
2. Install required dependencies:
   ```bash
   # For Ubuntu/Debian
   apt-get install txtar yq
   
   # For macOS
   brew install txtar yq
   ```
3. Clone the repository or copy the Sophon scripts to your machine

## Starting a New Project

### 1. Create Project Directory

```bash
mkdir my-project
cd my-project
```

### 2. Define Your Goals

Create a file with your requirements:

```bash
echo "Build a simple REST API with the following endpoints..." > requirements.txt
```

### 3. Initialize Sophon

```bash
/path/to/sophon-agent-init.sh "Implement the requirements in requirements.txt"
```

### 4. Run the Agent

```bash
/path/to/sophon-agent.sh
```

## Example: Hello World Web App

Let's create a simple Flask web application:

1. Create a project directory:
   ```bash
   mkdir flask-hello-world
   cd flask-hello-world
   ```

2. Create a requirements file:
   ```bash
   echo "Create a simple Flask web application with:
   1. A home page that displays 'Hello, World!'
   2. A '/about' page with basic information
   3. Proper project structure with templates
   4. Basic CSS styling
   5. Requirements file for dependencies" > requirements.txt
   ```

3. Initialize and run Sophon:
   ```bash
   /path/to/sophon-agent-init.sh "Create a Flask web application according to requirements.txt"
   /path/to/sophon-agent.sh
   ```

4. After completion, run the app:
   ```bash
   pip install -r requirements.txt
   python app.py
   ```

## Command Reference

### Initialization

```bash
sophon-agent-init.sh "<your goal here>"
```

Options:
- Set custom system prompt: Create `.h-sophon-agent-init` before running

### Agent Loop

```bash
sophon-agent.sh
```

Environment variables:
- `HIST_FILE`: History file (default: `.h-mk`)
- `CYCLE`: Starting cycle (default: 0)
- `ITERATIONS`: Maximum iterations (default: 10)

Example with custom configuration:
```bash
ITERATIONS=20 HIST_FILE=.h-custom sophon-agent.sh
```

## File Format Reference

Sophon uses the txtar format for file modifications:

```
-- path/to/file.txt --
Contents of file.txt
Complete file contents here

-- another/file.js --
function example() {
  return "Another file";
}
```

## Next Steps

- Read the full tutorial in `tutorial.md`
- Explore example projects in the `examples/` directory
- Check the README for advanced configuration options