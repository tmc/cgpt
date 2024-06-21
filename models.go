package cgpt

import (
	"fmt"

	"github.com/tmc/langchaingo/httputil"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// extend these to accept a debugMode flag

var constructors = map[string]func(modelName string, debugMode bool) (llms.Model, error){
	"openai": func(modelName string, debugMode bool) (llms.Model, error) {
		options := []openai.Option{openai.WithModel(modelName)}
		if debugMode {
			options = append(options, openai.WithHTTPClient(httputil.DebugHTTPClient))
		}
		return openai.New(options...)
	},
	"anthropic": func(modelName string, debugMode bool) (llms.Model, error) {
		options := []anthropic.Option{anthropic.WithModel(modelName)}
		if debugMode {
			options = append(options, anthropic.WithHTTPClient(httputil.DebugHTTPClient))
		}
		return anthropic.New(options...)
	},
	"ollama": func(modelName string, debugMode bool) (llms.Model, error) {
		options := []ollama.Option{ollama.WithModel(modelName)}
		if debugMode {
			options = append(options, ollama.WithHTTPClient(httputil.DebugHTTPClient))
		}
		return ollama.New(options...)
	},
}

func initializeModel(backend, modelName string, debugMode bool) (llms.Model, error) {
	constructor, ok := constructors[backend]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", backend)
	}
	return constructor(modelName, debugMode)
}
