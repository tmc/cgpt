# cgpt - Command-line interface for interacting with LLMs

## Overview

`cgpt` is a command-line interface for interacting with various language model APIs including:

- Anthropic (Claude)
- OpenAI (GPT models)
- Google (Gemini models)
- Ollama (local models)

## Installation

```bash
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

## Usage

Basic usage:

```bash
cgpt "How does garbage collection work in Go?"
```

With a specific system prompt:

```bash
cgpt -s "You are an expert in Go programming. Answer technically and in detail." "How does context.Context work?"
```

With a specific model:

```bash
cgpt --model claude-3-opus-20240229 "Write a Rust implementation of a binary search tree."
```

## Features

- Multi-backend support (Anthropic, OpenAI, Google, Ollama)
- History management
- System prompts
- Session management
- Streaming responses
- Prefill capabilities
- VIM integration

## Configuration

### Environment Variables

```
ANTHROPIC_API_KEY    # Anthropic API key
OPENAI_API_KEY       # OpenAI API key  
GOOGLE_API_KEY       # Google API key
OPENAI_BASE_URL      # Optional: custom base URL for OpenAI API
CGPT_BACKEND         # Default backend to use
CGPT_MODEL           # Default model to use
```

### Config File

Configuration can also be specified in a file at `~/.cgpt/config.yaml`:

```yaml
backend: anthropic
model: claude-3-haiku-20240307
stream: true
max_tokens: 2000
temperature: 0.7
```

## Command-line Options

```
USAGE:
  cgpt [options] [prompt]

OPTIONS:
  -b, --backend string      Backend to use (anthropic, openai, googleai, ollama, dummy) (default "anthropic")
  -c, --config string       Path to config file (default "$HOME/.cgpt/config.yaml")
  -d, --debug               Enable debug output
  -h, --help                Show this help message
  -i, --input string        Input from stdin (default "-")
  -m, --model string        Model to use
  -p, --prefill string      Prefill the model context with this text
  -s, --system string       System prompt
  -t, --max-tokens int      Maximum tokens to generate (default 4096)
  -T, --temperature float   Sampling temperature (default 0.05)
  --stream                  Stream output (default true)

HISTORY OPTIONS:
  --history-load string   Load history from file
  --history-save string   Save history to file

SESSION OPTIONS:
  --session string         Use named session
  --auto-session          Auto-create session name from conversation
  --project string         Project name for organizing sessions
```

## Examples

### Interactive Conversation

```bash
cgpt -s "You are a helpful assistant who speaks like a pirate."
> Tell me about the history of programming.
Arr me hearty! The tale o' programmin' be a long and windy one...
```

### Processing Files

```bash
cat mycode.go | cgpt "Explain this code and suggest improvements."
```

### Using Session Management

```bash
# Start a session with auto-naming
cgpt --auto-session "Let's learn about quantum computing."

# Continue a specific session
cgpt --session quantum-computing-basics "Explain superposition."
```

### Using History Files

```bash
cgpt --history-save quantum.yaml "What is quantum entanglement?"
# Later...
cgpt --history-load quantum.yaml "Can you explain that more simply?"
```

## VIM Integration

For Vim integration, see [vim/doc/cgpt.txt](vim/doc/cgpt.txt).

## Testing

The cgpt codebase uses two main testing systems:

### HTTP Recording/Replay

For tests that need to interact with remote APIs, we use a record/replay system:

```bash
# Record tests (requires API keys)
go test ./... -httprecord=".*"

# Run tests without making API calls (uses recorded responses)
go test ./...
```

### Terminal Session Testing

For testing interactive CLI functionality:

```bash
# Run tests with terminal session recording
go test ./... -termrecord=".*"

# Visualize a recorded terminal session
termcat testdata/term/basic_session.txtar
```

## License

[LICENSE](LICENSE)