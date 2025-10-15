# CGPT Hook Examples

This directory contains example hooks that demonstrate various use cases for the cgpt hook system.

## Hook Structure

Hooks can be defined in two ways:

1. **JSON Configuration**: Define multiple hooks in a `hooks.json` file
2. **Executable Files**: Place executable files in event-specific subdirectories

## Available Events

- `session-start`: Triggered when a cgpt session begins
- `session-end`: Triggered when a cgpt session ends
- `pre-completion`: Triggered before sending a request to the LLM
- `post-completion`: Triggered after receiving a response from the LLM
- `pre-save`: Triggered before saving conversation history
- `post-save`: Triggered after saving conversation history
- `pre-load`: Triggered before loading conversation history
- `post-load`: Triggered after loading conversation history
- `interactive-start`: Triggered when entering interactive mode
- `interactive-end`: Triggered when exiting interactive mode
- `user-input`: Triggered when user provides input in interactive mode
- `config-loaded`: Triggered after configuration is loaded
- `completion-error`: Triggered when a completion request fails

## Example Hooks

### JSON Configuration (`hooks.json`)

This file demonstrates various hook configurations:

- **log-conversation**: Logs completion events with timestamps
- **backup-important-conversations**: Creates backups of long conversations
- **cost-tracker**: Tracks API usage and costs
- **security-scan**: Scans for potential sensitive information before completion

### Executable Hooks

#### Session Start (`session-start/environment-check.sh`)

- Checks environment setup
- Creates necessary directories
- Validates API key configuration
- Checks for required utilities
- Logs session start

#### Pre-completion (`pre-completion/input-validator.sh`)

- Validates input message length
- Checks for potential prompt injection
- Warns about very short or very long inputs
- Demonstrates input security checking

#### Post-completion (`post-completion/response-processor.py`)

- Analyzes response characteristics (word count, code blocks, etc.)
- Detects programming languages and frameworks mentioned
- Logs analysis results for later review
- Demonstrates response processing and analysis

#### Post-save (`post-save/git-commit.sh`)

- Automatically commits conversation history to git
- Initializes git repository if needed
- Creates descriptive commit messages
- Demonstrates version control integration

## Hook Context

Hooks receive context through stdin as JSON containing:

```json
{
  "event": "pre-completion",
  "timestamp": "2025-01-XX...",
  "config": {
    "backend": "anthropic",
    "model": "claude-sonnet-4-20250514",
    ...
  },
  "messages": [...],
  "response": "...",
  "error": "...",
  "metadata": {...},
  "environment": {...},
  "workingDir": "/path/to/working/dir",
  "sessionId": "session-123"
}
```

## Security Features

The hook system includes several security features:

- **Command validation**: Dangerous commands are blocked
- **Sandboxed execution**: Hooks run with restricted permissions
- **Rate limiting**: Prevents excessive hook execution
- **Audit logging**: All hook executions are logged
- **Permission management**: Fine-grained control over hook execution

## Configuration

Hooks can be configured globally (`~/.cgpt/hooks/`) or per-project (`.cgpt/hooks/`).

Example configuration in `config.yaml`:

```yaml
hooks:
  enabled: true
  globalHooksDir: "~/.cgpt/hooks"
  projectHooksDir: ".cgpt/hooks"
  timeout: "30s"
  maxConcurrency: 5
  enableSandbox: true
  securityPolicy:
    allowNetworkAccess: false
    allowFileSystem: true
    maxExecutionTime: "30s"
    maxMemoryUsage: 104857600  # 100MB
    restrictedPaths:
      - "/etc"
      - "/sys"
      - "/proc"
  allowedCommands:
    - "/usr/bin/python3"
    - "/bin/bash"
    - "/usr/bin/jq"
  blockedCommands:
    - "rm"
    - "sudo"
    - "curl"
```

## Installation

1. Copy the desired hooks to your hooks directory:
   ```bash
   mkdir -p ~/.cgpt/hooks
   cp -r examples/hooks/* ~/.cgpt/hooks/
   ```

2. Make sure executable hooks have execute permissions:
   ```bash
   chmod +x ~/.cgpt/hooks/*/\*
   ```

3. Enable hooks in your cgpt configuration:
   ```yaml
   hooks:
     enabled: true
   ```

## Writing Custom Hooks

### Requirements

1. Hooks must be executable files or defined in `hooks.json`
2. Hooks receive context via stdin as JSON
3. Hooks should exit with code 0 for success, non-zero for failure
4. Use stderr for user-visible messages
5. Follow security best practices

### Best Practices

1. **Error Handling**: Always handle errors gracefully
2. **Timeouts**: Keep execution time short (< 30 seconds)
3. **Logging**: Log important events for debugging
4. **Security**: Validate inputs and avoid dangerous operations
5. **Performance**: Minimize resource usage
6. **Compatibility**: Work across different platforms when possible

### Example Hook Template

```bash
#!/bin/bash
# Hook description

# Read context from stdin
CONTEXT=$(cat)

# Process the context
# ... your hook logic here ...

# Output results (if any) to stdout
# Output messages to stderr for user visibility
echo "Hook completed successfully" >&2

# Exit with appropriate code
exit 0
```

## Troubleshooting

- Check hook permissions: `ls -la ~/.cgpt/hooks/*/`
- Verify hook syntax: Run hooks manually with test input
- Review audit logs: `cat ~/.cgpt/hooks/audit.log`
- Check cgpt logs for hook execution messages
- Test with `--verbose` flag for detailed output