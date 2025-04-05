package options

import (
	"net/http"
)

// InferenceProviderOptions contains options for model initialization.
type InferenceProviderOptions struct {
	// HTTPClient is the HTTP client to use for the model.
	HTTPClient *http.Client

	// UseLegacyMaxTokens controls whether to use max_tokens vs max_output_tokens for openai backends
	UseLegacyMaxTokens bool
}

// InferenceProviderOption is a function that modifies the model options.
type InferenceProviderOption func(*InferenceProviderOptions)
