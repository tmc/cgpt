package cgpt

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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
		assert.NoError(t, err)
		assert.Equal(t, "anthropic", cfg.Backend)
		assert.Equal(t, "claude-3-5-sonnet-20240620", cfg.Model)
		assert.Equal(t, 4000, cfg.MaxTokens)
		assert.Equal(t, 2*time.Minute, cfg.CompletionTimeout)
	})

	t.Run("ConfigFromFlags", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		fs.Set("backend", "openai")
		fs.Set("model", "gpt-3.5-turbo")
		cfg, err := LoadConfig("", fs)
		assert.NoError(t, err)
		assert.Equal(t, "openai", cfg.Backend)
		assert.Equal(t, "gpt-3.5-turbo", cfg.Model)
	})

	t.Run("ConfigFromEnv", func(t *testing.T) {
		resetViperAndEnv()
		os.Setenv("CGPT_BACKEND", "googleai")
		fs := createFlagSet()
		cfg, err := LoadConfig("", fs)
		assert.NoError(t, err)
		assert.Equal(t, "googleai", cfg.Backend)
		assert.Equal(t, "gemini-pro", cfg.Model)
	})

	t.Run("ConfigPriority", func(t *testing.T) {
		resetViperAndEnv()
		os.Setenv("CGPT_BACKEND", "googleai")
		fs := createFlagSet()
		fs.Set("backend", "openai")
		cfg, err := LoadConfig("", fs)
		assert.NoError(t, err)
		assert.Equal(t, "openai", cfg.Backend)
		assert.Equal(t, "gpt-4o", cfg.Model)
	})

	t.Run("CustomModel", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		fs.Set("backend", "openai")
		fs.Set("model", "gpt-4-32k")
		cfg, err := LoadConfig("", fs)
		assert.NoError(t, err)
		assert.Equal(t, "openai", cfg.Backend)
		assert.Equal(t, "gpt-4-32k", cfg.Model)
	})

	t.Run("InvalidBackend", func(t *testing.T) {
		resetViperAndEnv()
		fs := createFlagSet()
		fs.Set("backend", "invalid")
		cfg, err := LoadConfig("", fs)
		assert.NoError(t, err)
		assert.Equal(t, "invalid", cfg.Backend)
		assert.Equal(t, "", cfg.Model)
	})
}
