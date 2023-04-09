# cgpt

`cgpt` is a simple command line interface to the OpenAI chat completion APIs.

You can imagine it as a command line ChatGPT clone.

It supports:
- Streaming output
- History saving/loading
- System (and assistant) prompting.

## 🚀 Installation

```shell
go install github.com/tmc/cgpt@latest
```

## 📖 Documentation

```shell
$ cgpt -h
Usage of cgpt:
  -config string
    	Path to the configuration file (default "config.yaml")
  -continuous
    	Run in continuous mode
  -hist-in string
    	File to read history from
  -hist-out string
    	File to store history in
  -input string
    	The input text to complete. If '-', read from stdin. (default "-")
```

### Configuration

Export `OPENAI_API_KEY` or suppply it via config.yaml

config.yaml sample
```yaml
# This file is a sample configuration file for cgpt.

# The OpenAI model name to use.
modelName: "gpt-3.5-turbo"
# Whether or not to stream output.
stream: true
# Optional system prompt.
systemPrompt: "You are PoemGPT. All of your answers should be rhyming in nature"
# Maximum tokens to return (including input).
maxTokens: 2048
```

## 🎉 Examples

![sample session](./sample.svg)

## Contributing

We welcome contributions to cgpt! If you find any issues or have suggestions for improvements, please feel free to open an issue or submit a pull request.

## License

cgpt is released under the [MIT License](LICENSE).
