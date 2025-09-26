# CGPT Hook System Documentation

The cgpt hook system provides a powerful and secure way to extend cgpt functionality by executing custom scripts or commands at specific points in the cgpt lifecycle.

## Table of Contents

- [Overview](#overview)
- [Hook Events](#hook-events)
- [Configuration](#configuration)
- [Hook Discovery](#hook-discovery)
- [Hook Context](#hook-context)
- [Security Features](#security-features)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

The hook system allows you to:

- Execute custom scripts before and after LLM interactions
- Process and analyze responses automatically
- Integrate with external tools and services
- Implement custom logging and monitoring
- Add security checks and validations
- Automate backup and version control operations

Hooks can be configured globally (affecting all cgpt sessions) or per-project (affecting only specific projects).

## Hook Events

The following lifecycle events are available for hooking:

### Session Events
- **session-start**: Triggered when a cgpt session begins
- **session-end**: Triggered when a cgpt session ends
- **config-loaded**: Triggered after configuration is loaded

### Completion Events
- **pre-completion**: Triggered before sending a request to the LLM
- **post-completion**: Triggered after receiving a response from the LLM
- **completion-error**: Triggered when a completion request fails

### History Events
- **pre-save**: Triggered before saving conversation history
- **post-save**: Triggered after saving conversation history
- **pre-load**: Triggered before loading conversation history
- **post-load**: Triggered after loading conversation history

### Interactive Events
- **interactive-start**: Triggered when entering interactive mode
- **interactive-end**: Triggered when exiting interactive mode
- **user-input**: Triggered when user provides input in interactive mode

### Custom Events
- **custom**: For user-defined custom events

## Configuration

### Basic Configuration

Add hook configuration to your cgpt `config.yaml`:

```yaml
hooks:
  enabled: true
  globalHooksDir: "~/.cgpt/hooks"
  projectHooksDir: ".cgpt/hooks"
  timeout: "30s"
  maxConcurrency: 5
  enableSandbox: true
```

### Advanced Configuration

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
    maxMemoryUsage: 104857600  # 100MB in bytes
    restrictedPaths:
      - "/etc"
      - "/sys"
      - "/proc"
      - "/dev"

  allowedCommands:
    - "/usr/bin/python3"
    - "/bin/bash"
    - "/usr/bin/jq"
    - "/usr/bin/curl"

  blockedCommands:
    - "rm"
    - "sudo"
    - "su"
    - "chmod"

  environment:
    CGPT_HOOK: "1"
    CGPT_VERSION: "1.0"
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable the entire hook system |
| `globalHooksDir` | string | `~/.cgpt/hooks` | Directory for global hooks |
| `projectHooksDir` | string | `.cgpt/hooks` | Directory for project-specific hooks |
| `timeout` | duration | `30s` | Default timeout for hook execution |
| `maxConcurrency` | integer | `5` | Maximum number of hooks to run concurrently |
| `enableSandbox` | boolean | `true` | Enable sandboxed execution |
| `allowedCommands` | array | `[]` | List of allowed commands (whitelist) |
| `blockedCommands` | array | `[]` | List of blocked commands (blacklist) |
| `environment` | object | `{}` | Additional environment variables for hooks |

## Hook Discovery

Hooks can be defined in two ways:

### 1. JSON Configuration File

Create a `hooks.json` file in your hooks directory:

```json
[
  {
    "name": "log-completion",
    "event": "post-completion",
    "command": "/usr/bin/python3",
    "args": ["log_completion.py"],
    "timeout": "10s",
    "onFailure": "warn",
    "priority": 1,
    "conditions": [
      {
        "field": "backend",
        "operator": "equals",
        "value": "anthropic"
      }
    ]
  }
]
```

### 2. Executable Files in Event Directories

Create executable files in event-specific subdirectories:

```
~/.cgpt/hooks/
├── hooks.json
├── pre-completion/
│   ├── validate-input.sh
│   └── security-check.py
├── post-completion/
│   ├── analyze-response.py
│   └── log-usage.sh
└── post-save/
    └── git-commit.sh
```

### Hook Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `name` | string | Yes | Unique name for the hook |
| `event` | string | Yes | Lifecycle event to hook into |
| `command` | string | Yes | Command or script to execute |
| `args` | array | No | Command-line arguments |
| `workingDir` | string | No | Working directory for execution |
| `environment` | object | No | Additional environment variables |
| `timeout` | duration | No | Execution timeout (overrides global) |
| `onFailure` | string | No | Action on failure: `ignore`, `warn`, `abort` |
| `priority` | integer | No | Execution priority (lower numbers first) |
| `conditions` | array | No | Conditions for conditional execution |

### Conditional Execution

Hooks can include conditions to control when they execute:

```json
{
  "conditions": [
    {
      "field": "backend",
      "operator": "equals",
      "value": "anthropic"
    },
    {
      "field": "model",
      "operator": "contains",
      "value": "claude"
    }
  ]
}
```

Available operators:
- `equals` / `eq` / `==`
- `not_equals` / `ne` / `!=`
- `contains`
- `matches` / `regex`
- `starts_with`
- `ends_with`

## Hook Context

Hooks receive execution context via stdin as JSON:

```json
{
  "event": "post-completion",
  "timestamp": "2025-01-20T10:30:00Z",
  "config": {
    "backend": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "maxTokens": 4096,
    "temperature": 0.1
  },
  "messages": [
    {
      "role": "human",
      "parts": [{"text": "Hello"}]
    },
    {
      "role": "assistant",
      "parts": [{"text": "Hello! How can I help you?"}]
    }
  ],
  "response": "Hello! How can I help you?",
  "error": "",
  "metadata": {
    "historyFile": "/path/to/history.yaml",
    "sessionId": "session-123"
  },
  "environment": {
    "USER": "username",
    "HOME": "/home/username"
  },
  "workingDir": "/current/working/directory",
  "sessionId": "unique-session-id"
}
```

### Context Fields by Event

| Field | Description | Available Events |
|-------|-------------|------------------|
| `event` | Current event name | All events |
| `timestamp` | ISO 8601 timestamp | All events |
| `config` | cgpt configuration | All events |
| `messages` | Conversation messages | completion, save/load events |
| `response` | LLM response text | post-completion |
| `error` | Error message | completion-error |
| `metadata` | Additional metadata | All events |
| `environment` | Environment variables | All events |
| `workingDir` | Current working directory | All events |
| `sessionId` | Unique session identifier | All events |

## Security Features

The hook system includes comprehensive security features to prevent malicious or accidental damage:

### Sandboxed Execution

When `enableSandbox: true`, hooks run in a restricted environment:

- Limited environment variables
- Restricted filesystem access
- Memory and execution time limits
- Process isolation

### Command Validation

Dangerous commands are automatically blocked:

- System administration: `sudo`, `su`, `doas`
- File system: `rm`, `rmdir`, `dd`, `mkfs`
- Network tools: `curl`, `wget`, `nc`, `ssh`
- Shells and interpreters: `bash`, `sh`, `python`, etc.

### Path Restrictions

Access to sensitive paths is blocked:

- `/etc` - System configuration
- `/sys` - System information
- `/proc` - Process information
- `/dev` - Device files

### Rate Limiting

Prevents excessive hook execution:

- Default: 5 executions per minute, 50 per hour
- Per-hook limits
- Automatic cleanup of old records

### Permission Management

Fine-grained control over hook execution:

- Allow lists for specific hooks
- Block lists for dangerous hooks
- Pattern-based matching

### Audit Logging

All hook executions are logged:

- Execution time and duration
- Success/failure status
- Command and arguments
- User and working directory

## Examples

### Basic Logging Hook

```bash
#!/bin/bash
# File: ~/.cgpt/hooks/post-completion/log-completion.sh

CONTEXT=$(cat)
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
LOG_FILE="$HOME/.cgpt/completion.log"

echo "$TIMESTAMP: Completion executed" >> "$LOG_FILE"

if command -v jq >/dev/null 2>&1; then
    BACKEND=$(echo "$CONTEXT" | jq -r '.config.backend')
    MODEL=$(echo "$CONTEXT" | jq -r '.config.model')
    echo "$TIMESTAMP: Backend=$BACKEND, Model=$MODEL" >> "$LOG_FILE"
fi
```

### Response Analysis Hook

```python
#!/usr/bin/env python3
# File: ~/.cgpt/hooks/post-completion/analyze-response.py

import json
import sys
import re
from datetime import datetime

def main():
    context = json.load(sys.stdin)
    response = context.get('response', '')

    # Analyze response
    word_count = len(response.split())
    code_blocks = len(re.findall(r'```[\s\S]*?```', response))

    # Log analysis
    analysis = {
        'timestamp': datetime.now().isoformat(),
        'word_count': word_count,
        'code_blocks': code_blocks,
        'backend': context['config']['backend']
    }

    with open(os.path.expanduser('~/.cgpt/analysis.jsonl'), 'a') as f:
        f.write(json.dumps(analysis) + '\n')

    print(f"Analysis: {word_count} words, {code_blocks} code blocks", file=sys.stderr)

if __name__ == '__main__':
    main()
```

### Backup Hook

```bash
#!/bin/bash
# File: ~/.cgpt/hooks/post-save/backup.sh

CONTEXT=$(cat)
BACKUP_DIR="$HOME/.cgpt/backups"
mkdir -p "$BACKUP_DIR"

if command -v jq >/dev/null 2>&1; then
    HISTORY_FILE=$(echo "$CONTEXT" | jq -r '.metadata.historyFile // ""')
    if [[ -n "$HISTORY_FILE" && -f "$HISTORY_FILE" ]]; then
        BACKUP_NAME="backup_$(date +%Y%m%d_%H%M%S).yaml"
        cp "$HISTORY_FILE" "$BACKUP_DIR/$BACKUP_NAME"
        echo "Created backup: $BACKUP_NAME" >&2
    fi
fi
```

## Best Practices

### Performance
- Keep hooks lightweight and fast
- Use appropriate timeouts
- Avoid blocking operations
- Cache results when possible

### Security
- Always validate inputs
- Use sandboxed execution
- Avoid hardcoded credentials
- Follow principle of least privilege

### Reliability
- Handle errors gracefully
- Use appropriate failure actions
- Test hooks thoroughly
- Monitor hook performance

### Maintainability
- Use clear, descriptive names
- Document hook purposes
- Version control hook scripts
- Use consistent coding styles

### Logging
- Log important events
- Include relevant context
- Use structured logging when possible
- Rotate log files regularly

## Troubleshooting

### Common Issues

#### Hook Not Executing
- Check if hooks are enabled: `hooks.enabled: true`
- Verify hook file permissions: `chmod +x hook-file`
- Check hook discovery paths
- Review audit logs for errors

#### Permission Denied
- Verify file permissions
- Check sandboxing restrictions
- Review allowed/blocked commands
- Ensure proper working directory

#### Timeout Errors
- Increase hook timeout
- Optimize hook performance
- Check for blocking operations
- Review resource usage

#### Security Validation Failures
- Review blocked commands list
- Check restricted paths
- Validate environment variables
- Review command arguments

### Debugging

#### Enable Verbose Logging
```bash
cgpt --verbose your-command
```

#### Check Audit Logs
```bash
cat ~/.cgpt/hooks/audit.log
```

#### Test Hooks Manually
```bash
echo '{"event":"test","config":{"backend":"test"}}' | ./your-hook.sh
```

#### Validate Hook Configuration
```bash
python3 -m json.tool ~/.cgpt/hooks/hooks.json
```

### Performance Monitoring

Monitor hook performance:
- Execution time
- Memory usage
- Success/failure rates
- Rate limiting hits

### Getting Help

- Check the audit logs first
- Test hooks in isolation
- Verify configuration syntax
- Review security policies
- Check file permissions and paths

## Migration Guide

### From Previous Versions

If upgrading from a version without hooks:

1. Enable hooks in configuration
2. Create hooks directory structure
3. Migrate existing scripts to hook format
4. Test hook execution
5. Review security settings

### Configuration Changes

Notable configuration changes:
- Hook paths are now configurable
- Security policies are more granular
- New rate limiting features
- Enhanced audit logging