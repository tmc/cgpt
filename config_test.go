package cgpt

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestBackendDefaultModel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("CGPT_BACKEND", "dummy")

	def := Config{Stream: true, MaxTokens: 4096, Temperature: 0.05, MaxRetries: 3, RetryDelay: time.Second}
	tests := []struct {
		name, configYAML string
		env              map[string]string
		flags            []string
		want             Config
		wantLogs         string
	}{
		{
			name:     "flag backend uses its default model",
			flags:    []string{"--backend=dummy"},
			want:     Config{Backend: "dummy", Model: "dummy", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature, MaxRetries: def.MaxRetries, RetryDelay: def.RetryDelay},
			wantLogs: "cgpt: using default model for dummy backend: dummy",
		},
		{
			name:  "flag backend but explicit model flag preserved",
			flags: []string{"--backend=dummy", "--model=dummy-custom"},
			want:  Config{Backend: "dummy", Model: "dummy-custom", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature, MaxRetries: def.MaxRetries, RetryDelay: def.RetryDelay},
		},
		{
			name:  "flag backend but env model preserved",
			flags: []string{"--backend=dummy"},
			env:   map[string]string{"CGPT_MODEL": "dummy-custom"},
			want:  Config{Backend: "dummy", Model: "dummy-custom", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature, MaxRetries: def.MaxRetries, RetryDelay: def.RetryDelay},
		},
		{
			name:       "flag backend but config model preserved",
			configYAML: "model: dummy-custom",
			flags:      []string{"--backend=dummy"},
			want:       Config{Backend: "dummy", Model: "dummy-custom", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature, MaxRetries: def.MaxRetries, RetryDelay: def.RetryDelay},
		},
		{
			name:     "env backend uses its default model",
			env:      map[string]string{"CGPT_BACKEND": "dummy"},
			want:     Config{Backend: "dummy", Model: "dummy", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature, MaxRetries: def.MaxRetries, RetryDelay: def.RetryDelay},
			wantLogs: "cgpt: using default model for dummy backend: dummy",
		},
		{
			name:       "config backend uses its default model",
			configYAML: "backend: dummy",
			want:       Config{Backend: "dummy", Model: "dummy", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature, MaxRetries: def.MaxRetries, RetryDelay: def.RetryDelay},
			wantLogs:   "cgpt: using default model for dummy backend: dummy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.configYAML != "" {
				f, err := os.CreateTemp("", "config.*.yaml")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(f.Name())
				if _, err := f.WriteString(tt.configYAML); err != nil {
					t.Fatal(err)
				}
				f.Close()
				configPath = f.Name()
			}

			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.String("backend", "anthropic", "")
			fs.String("model", "", "")
			fs.String("config", configPath, "")
			fs.Bool("verbose", true, "")
			if configPath != "" {
				fs.Set("config", configPath)
			}
			if err := fs.Parse(tt.flags); err != nil {
				t.Fatal(err)
			}

			var stderr bytes.Buffer
			cfg, err := LoadConfig(configPath, &stderr, fs)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(*cfg, tt.want) {
				t.Errorf("Config = %+v, want %+v", *cfg, tt.want)
			}
			if tt.wantLogs != "" && !strings.Contains(stderr.String(), tt.wantLogs) {
				t.Errorf("Logs = %q, want to contain %q", stderr.String(), tt.wantLogs)
			}
		})
	}
}

func TestAutoBackendSelection(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		flags       []string
		want        string
		wantLogs    string
		description string
	}{
		{
			name: "no api keys available uses default",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "",
				"OPENAI_API_KEY":     "",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			want:        "anthropic", // falls back to default when no cloud API keys are available
			wantLogs:    "cgpt: no API keys found, using default backend: anthropic",
			description: "When no API keys are available, should fall back to default backend",
		},
		{
			name: "single api key available",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			want:        "openai",
			wantLogs:    "cgpt: auto-selected backend: openai",
			description: "When only one API key is available, should select that backend",
		},
		{
			name: "multiple api keys selects highest priority",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "sk-ant-test",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			want:        "anthropic", // highest priority
			wantLogs:    "cgpt: auto-selected backend: anthropic",
			description: "When multiple API keys are available, should select highest priority",
		},
		{
			name: "all api keys available selects anthropic",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "sk-ant-test",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "AI-test",
				"OPENROUTER_API_KEY": "sk-or-test",
			},
			want:        "anthropic", // highest priority
			wantLogs:    "cgpt: auto-selected backend: anthropic",
			description: "When all API keys are available, should select anthropic (highest priority)",
		},
		{
			name: "explicit backend flag overrides auto-selection",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "sk-ant-test",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			flags:       []string{"--backend=openai"},
			want:        "openai",
			description: "Explicit backend flag should override auto-selection",
		},
		{
			name: "explicit backend env overrides auto-selection",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "sk-ant-test",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
				"CGPT_BACKEND":       "openai",
			},
			want:        "openai",
			description: "Explicit backend env var should override auto-selection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables first
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv("GOOGLE_API_KEY", "")
			t.Setenv("OPENROUTER_API_KEY", "")
			t.Setenv("CGPT_BACKEND", "")

			// Set test environment
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.String("backend", "", "") // No default to trigger auto-selection
			fs.String("model", "", "")
			fs.String("config", "", "")
			fs.Bool("verbose", true, "")

			// Add --config flag to prevent loading actual config file
			allFlags := append(tt.flags, "--config=/nonexistent/path/to/config.yaml")
			if err := fs.Parse(allFlags); err != nil {
				t.Fatal(err)
			}

			var stderr bytes.Buffer
			cfg, err := LoadConfig("", &stderr, fs)
			if err != nil {
				t.Fatal(err)
			}

			if cfg.Backend != tt.want {
				t.Errorf("Backend = %q, want %q", cfg.Backend, tt.want)
			}
			if tt.wantLogs != "" && !strings.Contains(stderr.String(), tt.wantLogs) {
				t.Errorf("Logs = %q, want to contain %q", stderr.String(), tt.wantLogs)
			}
		})
	}
}

func TestDetectAvailableBackends(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "no keys",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "",
				"OPENAI_API_KEY":     "",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			want: []string{}, // no API keys available
		},
		{
			name: "single key",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			want: []string{"openai"}, // sorted by priority
		},
		{
			name: "multiple keys",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "sk-ant-test",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "",
				"OPENROUTER_API_KEY": "",
			},
			want: []string{"anthropic", "openai"}, // sorted by priority
		},
		{
			name: "all keys",
			env: map[string]string{
				"ANTHROPIC_API_KEY":  "sk-ant-test",
				"OPENAI_API_KEY":     "sk-test",
				"GOOGLE_API_KEY":     "AI-test",
				"OPENROUTER_API_KEY": "sk-or-test",
			},
			want: []string{"anthropic", "openai", "googleai", "openrouter"}, // sorted by priority
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables first
			t.Setenv("ANTHROPIC_API_KEY", "")
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv("GOOGLE_API_KEY", "")
			t.Setenv("OPENROUTER_API_KEY", "")

			// Set test environment
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got := detectAvailableBackends()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("detectAvailableBackends() = %v, want %v", got, tt.want)
			}
		})
	}
}
