package cgpt

import (
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
)

var constructors = map[string]func(modelName string) (llms.Model, error){
	"openai": func(modelName string) (llms.Model, error) {
		return openai.New(openai.WithModel(modelName))
	},
	"anthropic": func(modelName string) (llms.Model, error) {
		return anthropic.New(anthropic.WithModel(modelName))
	},
}

func initializeModel(backend, modelName string) (llms.Model, error) {
	constructor, ok := constructors[backend]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", backend)
	}
	return constructor(modelName)
}
