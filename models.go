package cgpt

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/tmc/cgpt/providers/xai"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

var defaultBackend = "anthropic" // Configurable via 'CGPT_BACKEND" (or via configuration files).

var defaultModels = map[string]string{
	"anthropic": "claude-3-7-sonnet-20250219",
	"openai":    "gpt-4o",
	"ollama":    "llama3.2",
	"googleai":  "gemini-pro",
	"xai":       "grok-3",
	"dummy":     "dummy",
}

// tokenLimits is a map of regex patterns to token limits for each backend.
// The key "*" is a catch-all for any patterns not explicitly defined.
// The value for each key is the maximum number of tokens allowed for a completion.
var tokenLimits = map[string]int{
	"*":                    4096,
	"googleai:*":           8192,
	"anthropic:.*sonnet.*": 8000,
}

// InferenceProviderOption configures model initialization.
type InferenceProviderOption func(*inferenceProviderOptions)

type inferenceProviderOptions struct {
	httpClient                     *http.Client
	openaiCompatUseLegacyMaxTokens bool
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) InferenceProviderOption {
	return func(mo *inferenceProviderOptions) {
		mo.httpClient = client
	}
}

// WithUseLegacyMaxTokens sets legacy max tokens behavior for OpenAI compatibility.
func WithUseLegacyMaxTokens(useLegacy bool) InferenceProviderOption {
	return func(mo *inferenceProviderOptions) {
		mo.openaiCompatUseLegacyMaxTokens = useLegacy
	}
}

// InitializeModel initializes the model with the given configuration and options.
func InitializeModel(cfg *Config, opts ...InferenceProviderOption) (llms.Model, error) {
	mo := &inferenceProviderOptions{
		httpClient: http.DefaultClient, // Default to http.DefaultClient if not specified
	}
	for _, opt := range opts {
		opt(mo)
	}

	constructor, ok := modelConstructors[cfg.Backend]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q", cfg.Backend)
	}

	return constructor(cfg, mo)
}

type modelConstructor func(*Config, *inferenceProviderOptions) (llms.Model, error)

var modelConstructors = map[string]modelConstructor{
	"openai": func(cfg *Config, mo *inferenceProviderOptions) (llms.Model, error) {
		options := []openai.Option{openai.WithModel(cfg.Model)}
		if cfg.OpenAIAPIKey != "" {
			options = append(options, openai.WithToken(cfg.OpenAIAPIKey))
		}
		if mo.httpClient != nil {
			options = append(options, openai.WithHTTPClient(mo.httpClient))
		}
		if mo.openaiCompatUseLegacyMaxTokens {
			options = append(options, openai.WithUseLegacyMaxTokens(true))
		}
		return openai.New(options...)
	},
	"anthropic": func(cfg *Config, mo *inferenceProviderOptions) (llms.Model, error) {
		options := []anthropic.Option{anthropic.WithModel(cfg.Model)}
		if cfg.AnthropicAPIKey != "" {
			options = append(options, anthropic.WithToken(cfg.AnthropicAPIKey))
		}
		if strings.Contains(cfg.Model, "sonnet") {
			options = append(options, anthropic.WithAnthropicBetaHeader(anthropic.MaxTokensAnthropicSonnet35))
		}
		if mo.httpClient != nil {
			options = append(options, anthropic.WithHTTPClient(mo.httpClient))
		}
		return anthropic.New(options...)
	},
	"ollama": func(cfg *Config, mo *inferenceProviderOptions) (llms.Model, error) {
		options := []ollama.Option{ollama.WithModel(cfg.Model)}
		if mo.httpClient != nil {
			options = append(options, ollama.WithHTTPClient(mo.httpClient))
		}
		return ollama.New(options...)
	},
	"googleai": func(cfg *Config, mo *inferenceProviderOptions) (llms.Model, error) {
		options := []googleai.Option{googleai.WithDefaultModel(cfg.Model)}
		if cfg.GoogleAPIKey != "" {
			options = append(options, googleai.WithAPIKey(cfg.GoogleAPIKey))
		}
		if mo.httpClient != nil {
			options = append(options, googleai.WithHTTPClient(mo.httpClient))
		}
		return googleai.New(context.TODO(), options...)
	},
	"xai": func(cfg *Config, mo *inferenceProviderOptions) (llms.Model, error) {
		opts := []xai.GrokOption{
			xai.WithModel(cfg.Model),
			// conversationID set dynamically in GenerateContent
		}
		
		// Add standard environment variable options
		if apiKey := os.Getenv("XAI_API_KEY"); apiKey != "" {
			opts = append(opts, xai.WithAPIKey(apiKey))
		}
		if sessionCookie := os.Getenv("XAI_SESSION_COOKIE"); sessionCookie != "" {
			opts = append(opts, xai.WithSessionCookie(sessionCookie))
		}
		
		// Process custom options from config
		for _, option := range cfg.XAIOptions {
			parts := strings.SplitN(option, "=", 2)
			if len(parts) != 2 {
				continue // Skip malformed options
			}
			
			optName, optValue := parts[0], parts[1]
			switch optName {
			case "WithRequireHTTP2":
				if optValue == "false" {
					opts = append(opts, xai.WithRequireHTTP2(false))
				} else if optValue == "true" {
					opts = append(opts, xai.WithRequireHTTP2(true))
				}
			}
			// Add more option handlers as needed
		}
		
		// Add custom HTTP client if provided
		if mo.httpClient != nil {
			opts = append(opts, xai.WithHTTPClient(mo.httpClient))
		}
		
		return xai.NewGrok3(opts...)
	},
	"dummy": func(cfg *Config, mo *inferenceProviderOptions) (llms.Model, error) {
		return NewDummyBackend()
	},
}
