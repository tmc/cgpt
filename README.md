# cgpt

`cgpt` is a simple command line interface (CLI) for interacting with OpenAI's chat completion APIs. It can be thought of as a command line ChatGPT clone.

## ✨ Features

- **Streaming Output**: Real-time responses as you type.
- **History Management**: Save and load conversation history.
- **Customizable Prompts**: Set system and assistant prompts.

## 🚀 Installation

To install `cgpt`, you'll need Go installed on your machine. Then, run:

```shell
go install github.com/tmc/cgpt/cmd/cgpt@latest
```

## 📖 Usage

Run `cgpt` with the `-h` flag to see available commands and options:

```shell
cgpt -h
```

Example output:
```shell
Usage of cgpt:
  -backend string
    	The backend to use (default "openai")
  -completions int
    	Number of completions (when running non-interactively with history)
  -config string
    	Path to the configuration file (default "config.yaml")
  -continuous
    	Run in continuous mode
  -in string
    	File to read completion history from
  -input string
    	The input text to complete. If '-', read from stdin. (default "-")
  -model string
    	The model to use (default "gpt-4o")
  -out string
    	File to store completion history in
  -stream
    	Stream results (default true)
```

## VIM Integration

`cgpt` can be used as a completion engine in Vim. To do this, you can use the following configuration:

```vim
    Plug 'tmc/cgpt', { 'rtp': 'vim', 'do': 'go install ./cmd/cgpt' }
```

### Configuration

To use `cgpt`, you need to provide your OpenAI API key. You can do this by either exporting it as an environment variable or specifying it in a configuration file (`config.yaml`).

Example `config.yaml`:

```yaml
# This file is a sample configuration file for cgpt.

# The OpenAI model name to use.
modelName: "gpt-4o"
# Whether or not to stream output.
stream: true
# Optional system prompt.
systemPrompt: "You are PoemGPT. All of your answers should be rhyming in nature."
# Maximum tokens to return (including input).
maxTokens: 2048
```

## 🎉 Examples

Below is an example of a session using `cgpt`. Make sure to replace placeholders with your actual values:

```shell
export OPENAI_API_KEY=your_openai_api_key
cgpt -config path/to/your/config.yaml
```

Here's a visual example of using `cgpt`:

![sample session](./sample.svg)

## 🤝 Contributing

We welcome contributions to `cgpt`! If you find any issues or have suggestions for improvements, please feel free to open an issue or submit a pull request.

## 📝 License

`cgpt` is released under the [MIT License](LICENSE).

## 🛠️ Development

To run `cgpt` locally for development:

1. Clone the repository:
    ```shell
    git clone https://github.com/tmc/cgpt.git
    cd cgpt
    ```

2. Install dependencies:
    ```shell
    go mod tidy
    ```

3. Build and run:
    ```shell
    go install ./cmd/cgpt
    ```

4. Run tests:
    ```shell
    go test ./...
    ```

Feel free to reach out for any questions or further assistance!

Happy hacking! 🚀
