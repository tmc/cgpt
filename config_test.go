package cgpt

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestLoadConfig(t *testing.T) {
	// Helper function to reset Viper and environment between tests
	resetViperAndEnv := func() {
		viper.Reset()
		os.Clearenv()
	}

	// Helper function to create a minimal flag set
	createFlagSet := func() *pflag.FlagSet {
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		fs.String("backend", "anthropic", "Backend to use")
		fs.String("model", "", "Model to use")
		fs.Bool("verbose", false, "Verbose output")
		return fs
	}

	t.Run("DefaultConfig", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		cfg, err := LoadConfig("", fs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if cfg.Backend != "anthropic" {
			t.Errorf("Expected Backend to be 'anthropic', got '%s'", cfg.Backend)
		}
		if cfg.Model != "claude-3-5-sonnet-20240620" {
			t.Errorf("Expected Model to be 'claude-3-5-sonnet-20240620', got '%s'", cfg.Model)
		}
		if cfg.MaxTokens != 4000 {
			t.Errorf("Expected MaxTokens to be 4000, got %d", cfg.MaxTokens)
		}
		if cfg.CompletionTimeout != 2*time.Minute {
			t.Errorf("Expected CompletionTimeout to be 2 minutes, got %v", cfg.CompletionTimeout)
		}

	})

	t.Run("ConfigFromFlags", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		fs.Set("backend", "openai")
		fs.Set("model", "gpt-3.5-turbo")
		cfg, err := LoadConfig("", fs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if cfg.Backend != "openai" {
			t.Errorf("Expected backend to be 'openai', but got '%s'", cfg.Backend)
		}
		if cfg.Model != "gpt-3.5-turbo" {
			t.Errorf("Expected model to be 'gpt-3.5-turbo', but got '%s'", cfg.Model)
		}

	})

	t.Run("ConfigFromEnv", func(t *testing.T) {
		resetViperAndEnv()
		os.Setenv("CGPT_BACKEND", "googleai")
		fs := createFlagSet()
		cfg, err := LoadConfig("", fs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if cfg.Backend != "googleai" {
			t.Errorf("Expected backend to be 'googleai', but got '%s'", cfg.Backend)
		}
		if cfg.Model != "gemini-pro" {
			t.Errorf("Expected model to be 'gemini-pro', but got '%s'", cfg.Model)
		}

	})

	t.Run("ConfigPriority", func(t *testing.T) {
		resetViperAndEnv()
		os.Setenv("CGPT_BACKEND", "googleai")
		fs := createFlagSet()
		fs.Set("backend", "openai")
		cfg, err := LoadConfig("", fs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if cfg.Backend != "openai" {
			t.Errorf("Expected backend to be 'openai', but got '%s'", cfg.Backend)
		}
		if cfg.Model != "gpt-4o" {
			t.Errorf("Expected model to be 'gpt-4o', but got '%s'", cfg.Model)
		}

	})

	t.Run("CustomModel", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		fs.Set("backend", "openai")
		fs.Set("model", "gpt-4-32k")
		cfg, err := LoadConfig("", fs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if cfg.Backend != "openai" {
			t.Errorf("Expected backend to be 'openai', but got '%s'", cfg.Backend)
		}
		if cfg.Model != "gpt-4-32k" {
			t.Errorf("Expected model to be 'gpt-4-32k', but got '%s'", cfg.Model)
		}

	})

	t.Run("InvalidBackend", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		fs.Set("backend", "invalid")
		cfg, err := LoadConfig("", fs)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if cfg.Backend != "invalid" {
			t.Errorf("Expected Backend to be 'invalid', but got '%s'", cfg.Backend)
		}
		if cfg.Model != "" {
			t.Errorf("Expected Model to be empty, but got '%s'", cfg.Model)
		}

	})
}
