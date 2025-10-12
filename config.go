package cgpt

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var defaultBackend = "anthropic" // Configurable via 'CGPT_BACKEND" (or via configuration files).

// backendPriority defines the order of preference for auto-selecting backends
// when multiple API keys are available. Higher priority values are preferred.
var backendPriority = map[string]int{
	"anthropic":  4, // Highest priority
	"openai":     3,
	"googleai":   2,
	"openrouter": 1,
	"ollama":     0, // Lowest priority (doesn't require API key)
	"dummy":      -1, // Only for testing
}

var defaultModels = map[string]string{
	"anthropic":  "claude-sonnet-4-20250514",
	"openai":     "gpt-5",
	"openrouter": "anthropic/claude-sonnet-4",
	"ollama":     "gpt-oss",
	"googleai":   "gemini-2.5-pro",
	"dummy":      "dummy",
}

// modelPatterns maps model name patterns to their corresponding backends
var modelPatterns = map[string]string{
	// OpenRouter models (provider/model format takes precedence)
	"anthropic/":  "openrouter",
	"openai/":     "openrouter",
	"google/":     "openrouter",
	"meta-llama/": "openrouter",
	"mistralai/":  "openrouter",
	"cohere/":     "openrouter",
	"perplexity/": "openrouter",
	"deepmind/":   "openrouter",
	"nvidia/":     "openrouter",

	// Anthropic models
	"claude": "anthropic",
	"opus":   "anthropic",
	"sonnet": "anthropic",
	"haiku":  "anthropic",

	// OpenAI models
	"gpt":     "openai",
	"davinci": "openai",
	"curie":   "openai",
	"babbage": "openai",
	"ada":     "openai",
	"o1":      "openai",
	"o3":      "openai",

	// Google models
	"gemini": "googleai",
	"bard":   "googleai",

	// Ollama models (common ones)
	"llama":     "ollama",
	"mistral":   "ollama",
	"mixtral":   "ollama",
	"phi":       "ollama",
	"qwen":      "ollama",
	"deepseek":  "ollama",
	"codellama": "ollama",

	// Dummy
	"dummy": "dummy",
}

// Common model aliases/shortcuts
var modelAliases = map[string]string{
	// Anthropic shortcuts
	"opus-4.1": "claude-opus-4-1-20250805",
	"opus-4":   "claude-opus-4-1-20250805",
	"opus":     "claude-opus-4-1-20250805",
	"sonnet-4": "claude-4-sonnet-20250522",
	"sonnet":   "claude-3-5-sonnet-20241022",
	"haiku":    "claude-3-haiku-20240307",

	// OpenAI shortcuts
	"gpt-5":       "gpt-5",
	"gpt-4":       "gpt-4-turbo-preview",
	"gpt-4-turbo": "gpt-4-turbo-preview",
	"gpt-3.5":     "gpt-3.5-turbo",
	"o1":          "o1-preview",
	"o1-mini":     "o1-mini",
	"o3":          "o3",
	"o3-mini":     "o3-mini",

	// Google shortcuts
	"gemini":     "gemini-pro",
	"gemini-pro": "gemini-pro",
	"gemini-1.5": "gemini-1.5-pro",

	// Ollama shortcuts
	"llama":     "llama3.2",
	"llama3":    "llama3.2",
	"mistral":   "mistral",
	"mixtral":   "mixtral",
	"codellama": "codellama",
	"deepseek":  "deepseek-coder",
}

// expandModelAlias expands a model alias to its full name
func expandModelAlias(model string) string {
	modelLower := strings.ToLower(model)
	if expanded, ok := modelAliases[modelLower]; ok {
		return expanded
	}
	return model
}

// detectBackendFromModel attempts to determine the backend based on the model name
func detectBackendFromModel(model string) (string, bool) {
	if model == "" {
		return "", false
	}

	modelLower := strings.ToLower(model)

	// First check for OpenRouter format (provider/model)
	// These patterns are more specific and should take precedence
	if strings.Contains(modelLower, "/") {
		for pattern, backend := range modelPatterns {
			if strings.HasSuffix(pattern, "/") && strings.HasPrefix(modelLower, pattern) {
				return backend, true
			}
		}
	}

	// Then check other patterns
	for pattern, backend := range modelPatterns {
		if !strings.HasSuffix(pattern, "/") && strings.Contains(modelLower, pattern) {
			return backend, true
		}
	}

	return "", false
}

// tokenLimits is a map of regex patterns to token limits for each backend.
// The key "*" is a catch-all for any patterns not explicitly defined.
// The value for each key is the maximum number of tokens allowed for a completion.
var tokenLimits = map[string]int{
	"*":                    4096,
	"google:*":             8192,
	"anthropic:.*opus-4.*": 8192,
	"anthropic:.*sonnet-4": 8192,
	"anthropic:.*sonnet.*": 8000,
}

type Config struct {
	Backend     string  `yaml:"backend"`
	Model       string  `yaml:"model"`
	Stream      bool    `yaml:"stream"`
	MaxTokens   int     `yaml:"maxTokens"`
	Temperature float64 `yaml:"temperature"`

	SystemPrompt string             `yaml:"systemPrompt"`
	LogitBias    map[string]float64 `yaml:"logitBias"`

	CompletionTimeout time.Duration `yaml:"completionTimeout"`

	Debug bool `yaml:"debug"`

	OpenAIAPIKey     string `yaml:"openaiAPIKey"`
	OpenRouterAPIKey string `yaml:"openrouterAPIKey"`
	AnthropicAPIKey  string `yaml:"anthropicAPIKey"`
	GoogleAPIKey     string `yaml:"googleAPIKey"`

	// Prompt caching (works with multiple backends)
	PromptCaching bool `yaml:"promptCaching"`

	// Thinking/reasoning mode for models that support it
	ThinkingMode        string `yaml:"thinkingMode"`
	ThinkingBudget      int    `yaml:"thinkingBudget"`
	ShowUsage           bool   `yaml:"showUsage"`
	ShowReasoning       bool   `yaml:"showReasoning"`
	InterleavedThinking bool   `yaml:"interleavedThinking"`

	// Retry configuration for API calls
	MaxRetries   int           `yaml:"maxRetries"`
	RetryDelay   time.Duration `yaml:"retryDelay"`
	DisableRetry bool          `yaml:"disableRetry"`

	// Hook system configuration
	Hooks *HookConfig `yaml:"hooks"`

	// TLS configuration
	InsecureSkipVerify bool `yaml:"insecureSkipVerify"`

	// API endpoint configuration
	BaseURL string `yaml:"baseURL"`
}

// ValidateThinkingConfig validates thinking mode configuration and returns warnings
func (c *Config) ValidateThinkingConfig() []string {
	var warnings []string

	// Check if thinking features are used with incompatible backends
	hasThinking := c.ThinkingBudget > 0 || (c.ThinkingMode != "" && c.ThinkingMode != "none") || c.InterleavedThinking

	// Define which backends support reasoning/thinking
	reasoningBackends := map[string]string{
		"anthropic":  "Extended Thinking",
		"openai":     "Reasoning Tokens (o1+ models)",
		"googleai":   "Thinking Budget (Gemini 2.5+ models)",
		"openrouter": "Reasoning Tokens (provider-dependent)",
		"ollama":     "Thinking Mode (DeepSeek-R1, QwQ, etc.)",
	}

	if hasThinking {
		if _, supported := reasoningBackends[c.Backend]; supported {
			// Supported backend - add specific notes
			if c.Backend == "openai" {
				warnings = append(warnings, fmt.Sprintf("Note: Using OpenAI Reasoning Tokens. Requires o1+ models (o1, o1-mini, o3, etc.)"))
			} else if c.Backend == "googleai" {
				warnings = append(warnings, fmt.Sprintf("Note: Using Google Gemini Thinking Budget. Requires Gemini 2.5+ models"))
			} else if c.Backend == "ollama" {
				warnings = append(warnings, fmt.Sprintf("Note: Using Ollama Thinking Mode. Requires reasoning models (deepseek-r1, qwq, etc.)"))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("Warning: Thinking features not supported by '%s' backend. Supported: %v", c.Backend, getBackendList(reasoningBackends)))
		}
	}

	// Validate thinking mode strings
	if c.ThinkingMode != "" && c.ThinkingMode != "none" {
		validModes := map[string]bool{
			"low":    true,
			"medium": true,
			"high":   true,
			"auto":   true,
		}
		if !validModes[c.ThinkingMode] {
			warnings = append(warnings, fmt.Sprintf("Warning: Invalid thinking mode '%s'. Valid options: none, low, medium, high, auto", c.ThinkingMode))
		}
	}

	// Backend-specific validation for supported backends
	if hasThinking {
		if _, supported := reasoningBackends[c.Backend]; supported {
			// Temperature validation (Anthropic-specific requirement)
			if c.Backend == "anthropic" && c.Temperature != 1.0 && c.Temperature != 0.05 {
				warnings = append(warnings, fmt.Sprintf("Warning: Temperature will be overridden to 1.0 (from %.2f) when thinking is enabled (Anthropic requirement)", c.Temperature))
			}

			// Budget validation (Anthropic has 1024 minimum, others may vary)
			if c.ThinkingBudget > 0 {
				if c.Backend == "anthropic" && c.ThinkingBudget < 1024 {
					warnings = append(warnings, fmt.Sprintf("Warning: Thinking budget %d is below Anthropic minimum of 1024 tokens, will be adjusted to 1024", c.ThinkingBudget))
				} else if c.Backend == "googleai" && c.ThinkingBudget < 0 && c.ThinkingBudget != -1 {
					warnings = append(warnings, fmt.Sprintf("Warning: Google Gemini thinking budget should be positive or -1 for auto-adjust"))
				}
			}
		}
	}

	// Check max_tokens vs thinking budget relationship (mainly for Anthropic)
	if c.Backend == "anthropic" && hasThinking && c.MaxTokens > 0 {
		effectiveBudget := c.ThinkingBudget
		if effectiveBudget == 0 && c.ThinkingMode != "" && c.ThinkingMode != "none" {
			effectiveBudget = 1024 // minimum default
		}
		// Normalize to API minimum
		if effectiveBudget > 0 && effectiveBudget < 1024 {
			effectiveBudget = 1024
		}
		if c.MaxTokens <= effectiveBudget {
			warnings = append(warnings, fmt.Sprintf("Note: max_tokens (%d) will be auto-adjusted to be greater than thinking budget (%d)", c.MaxTokens, effectiveBudget))
		}
	}

	return warnings
}

// getBackendList returns a formatted list of backend names
func getBackendList(backends map[string]string) []string {
	var names []string
	for name := range backends {
		names = append(names, name)
	}
	return names
}

// LoadConfig loads the configuration from various sources in the following order of precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Configuration file
// 4. Default values (lowest priority)
//
// The function performs the following steps:
// - Sets default values
// - Binds command-line flags
// - Loads environment variables
// - Reads the configuration file
// - Unmarshals the configuration into the Config struct
//
// If a config file is not found, it falls back to using defaults and flags.
// The --verbose flag can be used to print the final configuration.
func LoadConfig(path string, stderr io.Writer, flagSet *pflag.FlagSet) (*Config, error) {
	if flagSet == nil {
		flagSet = pflag.CommandLine
	}
	cfg := &Config{}
	v := viper.New()

	setupViper(v, flagSet)
	setupFlagNormalization(flagSet)

	// Read config file first
	if err := handleConfigFile(v, stderr, flagSet); err != nil {
		return nil, err
	}

	// Then bind flags (so they override config)
	if err := v.BindPFlags(flagSet); err != nil {
		return nil, fmt.Errorf("unable to bind flags: %w", err)
	}

	// Check if model is explicitly set anywhere before setting default
	hasModel := false
	modelName := ""
	if flagSet.Changed("model") {
		modelName = flagSet.Lookup("model").Value.String()
		if verbose, _ := flagSet.GetBool("verbose"); verbose {
			fmt.Fprintf(stderr, "cgpt: model set by flag: %s\n", modelName)
		}
		hasModel = true
	} else if isEnvSet("CGPT_MODEL") {
		modelName = os.Getenv("CGPT_MODEL")
		if verbose, _ := flagSet.GetBool("verbose"); verbose {
			fmt.Fprintf(stderr, "cgpt: model set by env: %s\n", modelName)
		}
		hasModel = true
	} else if v.InConfig("model") {
		modelName = v.GetString("model")
		if verbose, _ := flagSet.GetBool("verbose"); verbose {
			fmt.Fprintf(stderr, "cgpt: model set in config: %s\n", modelName)
		}
		hasModel = true
	}

	// Expand model alias if one was provided
	if hasModel && modelName != "" {
		expandedModel := expandModelAlias(modelName)
		if expandedModel != modelName {
			if verbose, _ := flagSet.GetBool("verbose"); verbose {
				fmt.Fprintf(stderr, "cgpt: expanded model alias %q to %q\n", modelName, expandedModel)
			}
			modelName = expandedModel
			v.Set("model", modelName)
		}
	}

	// Check if backend was explicitly set by user with a meaningful value
	backendFromFlag := flagSet.Changed("backend") && flagSet.Lookup("backend").Value.String() != ""
	backendFromEnv := isEnvSet("CGPT_BACKEND") && os.Getenv("CGPT_BACKEND") != ""
	backendFromConfig := v.InConfig("backend") && v.GetString("backend") != ""
	backendExplicit := backendFromFlag || backendFromEnv || backendFromConfig

	// Get backend (respecting precedence)
	backend := v.GetString("backend")

	// If model is set but backend is not explicitly set, try to detect backend from model name
	if hasModel && modelName != "" && !backendExplicit {
		if detectedBackend, ok := detectBackendFromModel(modelName); ok {
			backend = detectedBackend
			v.Set("backend", backend)
			if verbose, _ := flagSet.GetBool("verbose"); verbose {
				fmt.Fprintf(stderr, "cgpt: auto-detected backend %q from model %q\n", backend, modelName)
			}
			backendExplicit = true // Mark as resolved to avoid further auto-selection
		}
	}

	// If backend is still not explicitly set (empty or default), try auto-selection based on API keys
	verbose, _ := flagSet.GetBool("verbose")
	if !backendExplicit && backend == "" {
		if autoSelected, wasAutoSelected := autoSelectBackend(verbose, stderr); wasAutoSelected {
			backend = autoSelected
			v.Set("backend", backend)
		} else {
			// Fall back to default if auto-selection didn't find anything
			backend = defaultBackend
			v.Set("backend", backend)
			if verbose {
				fmt.Fprintf(stderr, "cgpt: no API keys found, using default backend: %s\n", defaultBackend)
			}
		}
	}

	if verbose, _ := flagSet.GetBool("verbose"); verbose {
		fmt.Fprintf(stderr, "cgpt: backend is %q\n", backend)
	}

	// Only set default model if no explicit model is set
	if !hasModel {
		if verbose, _ := flagSet.GetBool("verbose"); verbose {
			fmt.Fprintln(stderr, "cgpt: no model set, using default")
		}
		if defaultModel, ok := defaultModels[backend]; ok {
			v.Set("model", defaultModel)
			if verbose, _ := flagSet.GetBool("verbose"); verbose {
				fmt.Fprintf(stderr, "cgpt: using default model for %s backend: %s\n", backend, defaultModel)
			}
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal config: %w", err)
	}

	// Map the usage flag to ShowUsage field
	if v.IsSet("usage") {
		cfg.ShowUsage = v.GetBool("usage")
	}

	logConfig(cfg, stderr, flagSet)
	return cfg, nil
}

// Helper to check if an environment variable is set
func isEnvSet(key string) bool {
	_, exists := os.LookupEnv(key)
	return exists
}

// detectAvailableBackends scans environment variables to find which backends
// have API keys configured and returns them sorted by priority (highest first)
// Only considers backends that require API keys for automatic selection
func detectAvailableBackends() []string {
	available := []string{}

	// Check for each backend's API key
	// Note: ollama is excluded from auto-selection since it doesn't require an API key
	// and the auto-selection logic is specifically for API-key-based backends
	backendKeys := map[string]string{
		"anthropic":  "ANTHROPIC_API_KEY",
		"openai":     "OPENAI_API_KEY",
		"googleai":   "GOOGLE_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}

	for backend, envKey := range backendKeys {
		// Check if the environment variable is set and not empty
		if value := os.Getenv(envKey); value != "" {
			available = append(available, backend)
		}
	}

	// Sort by priority (highest first)
	sortBackendsByPriority(available)
	return available
}

// sortBackendsByPriority sorts a slice of backend names by their priority
// in descending order (highest priority first)
func sortBackendsByPriority(backends []string) {
	for i := 0; i < len(backends); i++ {
		for j := i + 1; j < len(backends); j++ {
			// Get priorities (default to -999 for unknown backends)
			iPrio, iExists := backendPriority[backends[i]]
			if !iExists {
				iPrio = -999
			}
			jPrio, jExists := backendPriority[backends[j]]
			if !jExists {
				jPrio = -999
			}

			// Swap if j has higher priority than i
			if jPrio > iPrio {
				backends[i], backends[j] = backends[j], backends[i]
			}
		}
	}
}

// autoSelectBackend attempts to automatically select a backend based on
// available API keys, returns the selected backend and whether auto-selection occurred
func autoSelectBackend(verbose bool, stderr io.Writer) (string, bool) {
	available := detectAvailableBackends()

	if verbose {
		fmt.Fprintf(stderr, "cgpt: detected available backends: %v\n", available)
	}

	if len(available) == 0 {
		return "", false // No auto-selection occurred
	}

	// Return the highest priority available backend
	selected := available[0]
	if verbose {
		fmt.Fprintf(stderr, "cgpt: auto-selected backend: %s (from available: %v)\n", selected, available)
	}

	return selected, true
}

func setupViper(v *viper.Viper, flagSet *pflag.FlagSet) {
	// Set defaults - NOTE: no default backend to allow auto-selection
	v.SetDefault("stream", true)
	v.SetDefault("temperature", 0.05)
	v.SetDefault("maxTokens", 4096)
	v.SetDefault("maxRetries", 3)
	v.SetDefault("retryDelay", time.Second)
	v.SetDefault("disableRetry", false)

	// Setup paths and env
	v.AddConfigPath("/etc/cgpt/")
	v.AddConfigPath("$HOME/.cgpt")
	v.AddConfigPath(".")
	v.SetConfigName("config")

	// Setup env vars
	v.SetEnvPrefix("CGPT")
	v.AutomaticEnv()
	v.BindEnv("openaiAPIKey", "OPENAI_API_KEY")
	v.BindEnv("openrouterAPIKey", "OPENROUTER_API_KEY")
	v.BindEnv("anthropicAPIKey", "ANTHROPIC_API_KEY")
	v.BindEnv("googleAPIKey", "GOOGLE_API_KEY")
	v.BindEnv("promptCaching", "PROMPT_CACHING", "ANTHROPIC_ENABLE_PROMPT_CACHING")

	// Set config file if specified in flags
	if flagConfigFilePath := flagSet.Lookup("config"); flagConfigFilePath != nil && flagConfigFilePath.Changed {
		v.SetConfigFile(flagConfigFilePath.Value.String())
	}
}
func logConfigSources(v *viper.Viper, stderr io.Writer) {
	for _, key := range []string{"backend", "model"} {
		var source string
		switch {
		case v.InConfig(key):
			source = "config file"
		case os.Getenv("CGPT_"+strings.ToUpper(key)) != "":
			source = "environment"
		case v.IsSet(key):
			source = "flag"
		default:
			source = "default"
		}
		fmt.Fprintf(stderr, "cgpt: using %s from %s: %s\n", key, source, v.GetString(key))
	}
}

func setupFlagNormalization(flagSet *pflag.FlagSet) {
	normalizeFunc := flagSet.GetNormalizeFunc()
	flagSet.SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		result := normalizeFunc(fs, name)
		name = strings.ReplaceAll(string(result), "-", "")
		return pflag.NormalizedName(name)
	})
}

func setMaxTokens(cfg *Config) {
	maxTokens := tokenLimits["*"]
	backendModel := cfg.Backend + ":" + cfg.Model

	for pattern, limit := range tokenLimits {
		if pattern == "*" {
			continue
		}
		if matched, _ := regexp.MatchString(pattern, backendModel); matched {
			maxTokens = limit
			break
		}
	}

	if cfg.MaxTokens == 0 || cfg.MaxTokens > maxTokens {
		cfg.MaxTokens = maxTokens
	}
}
func handleConfigFile(v *viper.Viper, stderr io.Writer, flagSet *pflag.FlagSet) error {
	if configFlag := flagSet.Lookup("config"); configFlag != nil && configFlag.Changed {
		configFile := configFlag.Value.String()
		if verbose, _ := flagSet.GetBool("verbose"); verbose {
			fmt.Fprintf(stderr, "cgpt: trying to read config file: %s\n", configFile)
		}

		// Check if file exists and is readable
		if _, err := os.Stat(configFile); err != nil {
			if verbose, _ := flagSet.GetBool("verbose"); verbose {
				fmt.Fprintf(stderr, "cgpt: config file %s not accessible: %v\n", configFile, err)
			}
			return nil
		}

		v.SetConfigFile(configFile)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if verbose, _ := flagSet.GetBool("verbose"); verbose {
				fmt.Fprintln(stderr, "cgpt: config file not found, using defaults")
			}
			return nil
		}
		return fmt.Errorf("unable to read config file: %w", err)
	}

	if verbose, _ := flagSet.GetBool("verbose"); verbose {
		fmt.Fprintf(stderr, "cgpt: successfully read config from %s\n", v.ConfigFileUsed())
	}
	return nil
}

func logConfig(cfg *Config, stderr io.Writer, flagSet *pflag.FlagSet) {
	if verbose, _ := flagSet.GetBool("verbose"); verbose {
		fmt.Fprint(stderr, "cgpt-config: ")
		json.NewEncoder(stderr).Encode(cfg)
	}
}
