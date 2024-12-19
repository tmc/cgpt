# cgpt

cgpt is a command-line tool for interacting with Large Language Models (LLMs) using various backends.

## Features

- Supports multiple backends: Anthropic, OpenAI, Ollama, and Google AI
- Interactive mode for continuous conversations
- Streaming output
- History management
- Configurable via YAML file and environment variables
- Vim plugin for easy integration

## Prerequisites

- Go 1.23 or higher (required for standard library packages like `cmp`, `log/slog`, and `slices`)
- One of the following API keys:
  - Anthropic API key
  - OpenAI API key
  - Google AI API key

## Installation

### Using Homebrew

```shell
brew install tmc/tap/cgpt
```

### From Source

cgpt is written in Go. To build from source:

1. Install Go 1.23 or higher:
   - Visit the [Go installation instructions](https://golang.org/doc/install)
   - Verify your Go installation and version:
     ```shell
     go version
     ```

2. Set up your Go environment:
   ```shell
   # Add Go binary directory to PATH if not already done
   echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc  # or ~/.zshrc for zsh
   source ~/.bashrc  # or source ~/.zshrc for zsh
   ```

3. Install cgpt:
   ```shell
   go install github.com/tmc/cgpt/cmd/cgpt@latest
   ```

4. Verify installation:
   ```shell
   cgpt --version
   ```

### From GitHub Releases

Download the latest release from the [GitHub Releases page](https://github.com/tmc/cgpt/releases).

## Usage

```
cgpt [flags]
```

### Flags

- `-b, --backend string`: The backend to use (default "anthropic")
- `-m, --model string`: The model to use (default "claude-3-5-sonnet-20241022")
- `-i, --input string`: Direct string input (overrides -f)
- `-f, --file string`: Input file path. Use '-' for stdin (default "-")
- `-c, --continuous`: Run in continuous mode (interactive)
- `-s, --system-prompt string`: System prompt to use
- `-p, --prefill string`: Prefill the assistant's response
- `-I, --history-load string`: File to read completion history from
- `-O, --history-save string`: File to store completion history in
- `--config string`: Path to the configuration file (default "config.yaml")
- `-v, --verbose`: Verbose output
- `--debug`: Debug output
- `-n, --completions int`: Number of completions (when running non-interactively with history)
- `-t, --max-tokens int`: Maximum tokens to generate (default 8000)
- `--completion-timeout duration`: Maximum time to wait for a response (default 2m0s)

## Configuration

### API Keys

Before using cgpt, you need to set up your API keys. Choose one or more of the following:

```bash
# Anthropic (recommended default)
export ANTHROPIC_API_KEY='your-key-here'

# OpenAI
export OPENAI_API_KEY='your-key-here'

# Google AI
export GOOGLE_API_KEY='your-key-here'
```

For persistent configuration, add these to your shell's configuration file (~/.bashrc, ~/.zshrc, etc.).

### Configuration File

cgpt can be configured using a YAML file. By default, it looks for `config.yaml` in the current directory. You can specify a different configuration file using the `--config` flag.

Example `config.yaml`:

```yaml
backend: "anthropic"
model: "claude-3-5-sonnet-20241022"
stream: true
maxTokens: 2048
systemPrompt: "You are a helpful assistant."
```

## Vim Plugin

cgpt includes a Vim plugin for easy integration. To use it, copy the `vim/plugin/cgpt.vim` file to your Vim plugin directory.

### Vim Plugin Usage

1. Visually select the text you want to process with cgpt.
2. Press `cg` or use the `:CgptRun` command to run cgpt on the selected text.
3. The output will be appended after the visual selection.

### Vim Plugin Configuration

- `g:cgpt_backend`: Set the backend for cgpt (default: 'anthropic')
- `g:cgpt_model`: Set the model for cgpt (default: 'claude-3-5-sonnet-20241022')
- `g:cgpt_system_prompt`: Set the system prompt for cgpt
- `g:cgpt_config_file`: Set the path to the cgpt configuration file
- `g:cgpt_include_filetype`: Include the current filetype in the prompt (default: 0)

## Troubleshooting

### Common Issues

1. **Old Go Version**
   - Error: Package not found errors mentioning `cmp`, `log/slog`, or `slices`
   - Solution: Upgrade to Go 1.23 or higher

2. **Missing API Keys**
   - Error: Authentication errors or "API key not found"
   - Solution: Set the appropriate environment variable for your chosen backend

3. **Configuration File Issues**
   - Error: "Config file not found" or YAML parsing errors
   - Solution: Ensure your config.yaml is properly formatted and in the correct location

For additional help, please check the [GitHub Issues](https://github.com/tmc/cgpt/issues) page.

## Examples

```bash
# Simple query
echo "Explain quantum computing" | cgpt

# Interactive mode
cgpt -c

# Use a specific backend and model
cgpt -b openai -m gpt-4 -i "Translate 'Hello, world!' to French"

# Load and save history
cgpt -I input_history.yaml -O output_history.yaml -i "Continue the conversation"
```

## License

This project is licensed under the ISC License. See the [LICENSE](LICENSE) file for details.

This README provides an overview of the cgpt tool, including its features, installation instructions, usage examples, configuration options, and information about the Vim plugin. It also includes details about the supported backends and environment variables for API keys.

Happy hacking! 🚀
