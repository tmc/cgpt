# cgpt

**cgpt** is a powerful, flexible command-line tool for interacting with Large Language Models (LLMs) from multiple AI providers. Whether you're a developer, researcher, writer, or just curious about AI, cgpt provides an intuitive interface to harness the power of modern language models.

## ✨ Key Features

- **🔌 Multiple AI Providers**: Anthropic Claude, OpenAI GPT, Google Gemini, Ollama, and OpenRouter
- **💬 Interactive Conversations**: Full-featured chat mode with history persistence
- **🚀 Streaming Responses**: Real-time output as the AI generates responses
- **📚 Session Management**: Advanced history tracking with automatic metadata and fork detection
- **⚙️ Highly Configurable**: YAML config files, environment variables, and extensive CLI options
- **🧠 Advanced AI Features**: Reasoning modes, prompt caching, usage tracking, and more
- **📝 Vim Integration**: Built-in Vim plugin for seamless editor workflows
- **🔄 Robust Error Handling**: Automatic retries and comprehensive error reporting

## 🚀 Quick Start

### 1. Install cgpt

```bash
# macOS/Linux (Homebrew - Recommended)
brew install tmc/tap/cgpt

# Or install with Go
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

### 2. Set up your API key

```bash
# Choose your preferred AI provider
export ANTHROPIC_API_KEY="your-anthropic-key"  # Recommended
export OPENAI_API_KEY="your-openai-key"
export GOOGLE_API_KEY="your-google-key"
```

### 3. Start using cgpt

```bash
# Simple question
echo "Explain quantum computing in simple terms" | cgpt

# Interactive mode
cgpt -c

# Analyze a file
cgpt -f script.py -s "Review this code for bugs"
```

## 📖 Usage Examples

### Basic Usage
```bash
# Ask questions directly
cgpt -i "What are the benefits of renewable energy?"

# Use different models
cgpt -m gpt-4 -i "Write a Python function for sorting"

# Custom system prompts
cgpt -s "You are a helpful coding assistant" -i "Debug this error"
```

### Advanced Features
```bash
# Interactive session with history
cgpt -c -H auto

# Continue previous conversation
cgpt -C

# Enable reasoning mode (Claude 4+)
cgpt --thinking-mode high -i "Solve this complex problem"

# Show usage and costs
cgpt --usage -i "Summarize this document" -f report.txt
```

### 📚 History Management

cgpt provides sophisticated session management:
- **Auto-generated sessions**: `cgpt -H auto` creates timestamped files in `~/.cgpt/history/sessions/`
- **Session metadata**: Includes creation time, descriptions, and parent session tracking
- **Fork tracking**: `cgpt -I old.yaml -O new.yaml` tracks conversation branches
- **Quick continuation**: `cgpt -C` resumes your most recent session
- **Smart descriptions**: Auto-generates session descriptions from your conversations

## 📋 Prerequisites

- **Go 1.23+** (if building from source) - Required for modern Go standard library
- **API Key** from at least one provider:
  - [Anthropic](https://console.anthropic.com/) (Claude models - recommended)
  - [OpenAI](https://platform.openai.com/) (GPT models)
  - [Google AI](https://aistudio.google.com/) (Gemini models)
  - [OpenRouter](https://openrouter.ai/) (Access to multiple providers)

## 🛠️ Installation

### Option 1: Homebrew (Recommended)

```bash
brew install tmc/tap/cgpt
```

### Option 2: Go Install

```bash
# Requires Go 1.23+
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

### Option 3: Download Binary

Download the latest release from [GitHub Releases](https://github.com/tmc/cgpt/releases).

### Option 4: Build from Source

```bash
git clone https://github.com/tmc/cgpt.git
cd cgpt
go build -o cgpt ./cmd/cgpt
```

### Verify Installation

```bash
cgpt --version
cgpt --help
```

## 🎯 Core Usage

```bash
cgpt [flags] [input...]
```

### Essential Flags

| Flag | Description | Example |
|------|-------------|----------|
| `-i, --input` | Direct text input | `cgpt -i "Hello, world!"` |
| `-f, --file` | Read from file | `cgpt -f document.txt` |
| `-c, --continuous` | Interactive chat mode | `cgpt -c` |
| `-s, --system-prompt` | Set AI behavior | `cgpt -s "You are a helpful teacher"` |
| `-m, --model` | Choose AI model | `cgpt -m gpt-4` |
| `-b, --backend` | Choose AI provider | `cgpt -b openai` |
| `-H, --history` | Session management | `cgpt -H auto` |
| `-C, --continue` | Resume last session | `cgpt -C` |
| `--usage` | Show costs/tokens | `cgpt --usage` |

### Model Selection

cgpt automatically detects the right provider based on model name:

```bash
# Anthropic models
cgpt -m claude-sonnet-4      # Latest Claude Sonnet
cgpt -m claude-haiku-3       # Fast, cost-effective
cgpt -m claude-opus-4        # Most capable

# OpenAI models
cgpt -m gpt-4               # GPT-4 Turbo
cgpt -m o1                  # Reasoning model
cgpt -m gpt-3.5             # Cost-effective

# Google models
cgpt -m gemini-pro          # Google's flagship
cgpt -m gemini-1.5          # Extended context

# OpenRouter (provider/model format)
cgpt -m anthropic/claude-sonnet-4
cgpt -m openai/gpt-4
```

## ⚙️ Configuration

### Environment Variables

```bash
# API Keys (choose your provider)
export ANTHROPIC_API_KEY="your-anthropic-key"      # Recommended
export OPENAI_API_KEY="your-openai-key"
export GOOGLE_API_KEY="your-google-key"
export OPENROUTER_API_KEY="your-openrouter-key"

# Default settings (optional)
export CGPT_MODEL="claude-sonnet-4-20250514"
export CGPT_BACKEND="anthropic"
export CGPT_MAX_TOKENS=8000
```

### Configuration File

Create `config.yaml` in your working directory or `~/.cgpt/config.yaml`:

```yaml
# Basic settings
backend: "anthropic"
model: "claude-sonnet-4-20250514"
maxTokens: 8000
temperature: 0.05
stream: true

# Advanced features
promptCaching: true          # Reduce costs
showUsage: true             # Track spending
thinkingMode: "medium"      # Enable reasoning

# System behavior
systemPrompt: "You are a helpful AI assistant."
debug: false
verbose: false

# Retry configuration
maxRetries: 3
retryDelay: "1s"
```

## 🎛️ Advanced Features

### Reasoning and Thinking Modes

For supported models (Claude 4+, o1+, Gemini 2.5+):

```bash
# Enable step-by-step reasoning
cgpt --thinking-mode high -i "Solve this complex problem"

# Set explicit token budget for thinking
cgpt --thinking-budget 2048 -i "Analyze this system"

# Show the AI's reasoning process
cgpt --show-reasoning -i "Debug this algorithm"
```

### Cost Optimization

```bash
# Enable prompt caching (reduces costs for repeated prompts)
cgpt --prompt-caching -s "You are a code reviewer..." -f code1.py
cgpt --prompt-caching -s "You are a code reviewer..." -f code2.py

# Track usage and costs
cgpt --usage -i "Analyze this data" -f large_dataset.csv
```

### Session Workflows

```bash
# Start a project session
cgpt -H project_session.yaml -c -s "You are helping me build a web app"

# Fork a conversation to try different approaches
cgpt -I main_session.yaml -O alternative_approach.yaml -i "Let's try a different method"

# Resume exactly where you left off
cgpt -C
```

## 📝 Vim Integration

cgpt includes a powerful Vim plugin:

### Installation
```bash
# Copy plugin to your Vim directory
cp vim/plugin/cgpt.vim ~/.vim/plugin/
```

### Usage
1. **Visual selection**: Select text and press `cg`
2. **Command mode**: `:CgptRun` to process selected text
3. **Custom prompts**: Set `g:cgpt_system_prompt` in your vimrc

### Configuration
```vim
" In your ~/.vimrc
let g:cgpt_backend = 'anthropic'
let g:cgpt_model = 'claude-sonnet-4-20250514'
let g:cgpt_system_prompt = 'You are a code review expert'
let g:cgpt_include_filetype = 1
```

## 🔧 Troubleshooting

### Quick Diagnostics

```bash
# Check your setup
cgpt --version                    # Verify installation
cgpt --verbose -i "test"         # Check configuration loading
cgpt --debug -i "test"           # Full diagnostic output
```

### Common Issues

| Problem | Symptoms | Solution |
|---------|----------|----------|
| **API Key Missing** | Authentication errors | `export ANTHROPIC_API_KEY="your-key"` |
| **Old Go Version** | Build errors with `cmp`, `slog` | Upgrade to Go 1.23+ |
| **Network Issues** | Timeouts, connection failures | Check internet, try `--completion-timeout 5m` |
| **Model Not Found** | "Model not available" | Verify model name, check API access |
| **Config Problems** | YAML parsing errors | Validate config syntax |
| **Truncated Output** | Incomplete responses | Increase `--max-tokens 16000` |

### Getting Help

1. **Built-in help**: `cgpt --help`, `cgpt --examples`
2. **Verbose mode**: `cgpt --verbose` for detailed info
3. **Debug mode**: `cgpt --debug` for full diagnostics
4. **GitHub issues**: [Report bugs or ask questions](https://github.com/tmc/cgpt/issues)
5. **Documentation**: See `USER_GUIDE.md` for comprehensive info

## 📚 More Resources

- **[USER_GUIDE.md](USER_GUIDE.md)**: Comprehensive usage guide with detailed examples
- **[CONFIGURATION.md](CONFIGURATION.md)**: Complete configuration reference
- **Advanced examples**: `cgpt --show-advanced-usage all`
- **Quick examples**: `cgpt --examples`

## 🤝 Contributing

We welcome contributions! Please see our contributing guidelines and:

- 🐛 **Report bugs**: [GitHub Issues](https://github.com/tmc/cgpt/issues)
- 💡 **Suggest features**: [GitHub Discussions](https://github.com/tmc/cgpt/discussions)
- 🔧 **Submit PRs**: Fork, branch, and submit pull requests
- 📖 **Improve docs**: Help us make the documentation better

## 📄 License

This project is licensed under the ISC License. See [LICENSE](LICENSE) for details.

---

**cgpt** - Bringing the power of AI to your command line. Made with ❤️ by the open source community.

[![GitHub stars](https://img.shields.io/github/stars/tmc/cgpt?style=social)](https://github.com/tmc/cgpt/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/tmc/cgpt)](https://github.com/tmc/cgpt/issues)
[![Go Report Card](https://goreportcard.com/badge/github.com/tmc/cgpt)](https://goreportcard.com/report/github.com/tmc/cgpt)
