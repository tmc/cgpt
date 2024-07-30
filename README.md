# cgpt: Your Command-Line AI Assistant

cgpt is a simple yet powerful command-line tool for interacting with Large Language Models (LLMs). It's designed to be your AI-powered sidekick right in your terminal! 💻✨

## 🛠 Installation

### Prerequisites

Before installing cgpt, you'll need to have Go installed on your system. Here's how to do it:

<details>
<summary>Installing Go</summary>

#### macOS (using Homebrew)

If you're on macOS and have Homebrew installed, you can easily install Go with:

```bash
brew install go
```

#### Other systems

1. Visit the [official Go download page](https://golang.org/dl/).
2. Download the appropriate package for your operating system.
3. Follow the installation instructions for your platform.

After installation, verify Go is correctly installed by running:

```bash
go version
```

This should display the installed Go version.

</details>

### Installing cgpt

Once you have Go installed, you can install cgpt using the following command:

```bash
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

This will download, compile, and install the `cgpt` binary in your `$GOPATH/bin` directory.

## 🚀 Usage

Here are some quick examples to get you started:

```bash
# Ask a question
echo "how should I interpret the output of nvidia-smi?" | cgpt

# Get a quick explanation
echo "explain plan 9 in one sentence" | cgpt
```

For more detailed usage instructions and options, run:

```bash
cgpt --help
```

## 🎛 Configuration

cgpt can be configured using a YAML file. By default, it looks for `config.yaml` in the current directory. You can specify a different config file using the `--config` flag.

Example `config.yaml`:

```yaml
backend: "anthropic"
modelName: "claude-3-5-sonnet-20240620"
stream: true
maxTokens: 2048
systemPrompt: "You are a helpful programming assistant."
```

## 🌟 Features

- 🔄 Supports multiple AI backends (OpenAI, Anthropic, Google AI, Ollama)
- 🔧 Configurable via YAML file or command-line flags
- 🖥 Interactive mode for continuous conversations
- 📜 History saving and loading
- 🔍 Verbose and debug modes for troubleshooting

## 🤝 Contributing

Contributions are welcome! Feel free to submit issues or pull requests.

## 📄 License

cgpt is released under the ISC License. See the [LICENSE](LICENSE) file for details.
