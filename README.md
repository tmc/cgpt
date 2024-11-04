# cgpt

cgpt is a command-line tool for interacting with Large Language Models (LLMs) using various backends.

## Features

- Supports multiple backends: Anthropic, OpenAI, Ollama, and Google AI
- Interactive mode for continuous conversations
- Streaming output
- History management
- Configurable via YAML file and environment variables
- Vim plugin for easy integration


# Installation

## Using Homebrew

```shell
brew install tmc/tap/cgpt
```

## From Source

cgpt is written in Go. To build from source, you need to have Go installed on your system. See the [Go installation instructions](https://golang.org/doc/install) for more information.

<details>
<summary>Quickstart Guide for Brew (including Go setup)</summary>

1. Install Homebrew:
   ```bash
   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
   ```

2. Install Go:
   ```bash
   brew install go
   ```

3. Add Go binary directory to PATH:
   ```bash
   echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.zshrc
   # Or if using bash:
   # echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bash_profile
   ```

4. Reload your shell configuration:
   ```bash
   source ~/.zshrc
   # Or if using bash:
   # source ~/.bash_profile
   ```
</details>

Once Go is set up, you can install cgpt from source:

```shell
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

## From GitHub Releases

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

cgpt can be configured using a YAML file. By default, it looks for `config.yaml` in the current directory. You can specify a different configuration file using the `--config` flag.

Example `config.yaml`:

```yaml
backend: "anthropic"
model: "claude-3-5-sonnet-20241022"
stream: true
maxTokens: 2048
systemPrompt: "You are a helpful assistant."
```

## Environment Variables

- `OPENAI_API_KEY`: OpenAI API key
- `OPENAI_BASE_URL`: Override for OpenAI API Base URL
- `ANTHROPIC_API_KEY`: Anthropic API key
- `GOOGLE_API_KEY`: Google AI API key

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

```

This README provides an overview of the cgpt tool, including its features, installation instructions, usage examples, configuration options, and information about the Vim plugin. It also includes details about the supported backends and environment variables for API keys.

Happy hacking! 🚀
```
