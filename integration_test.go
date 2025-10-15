package cgpt

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

// TestIntegration_EndToEndCompletion tests the complete flow from input to output
func TestIntegration_EndToEndCompletion(t *testing.T) {

	tests := []struct {
		name           string
		input          string
		backend        string
		model          string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "basic dummy completion",
			input:          "Hello, world!",
			backend:        "dummy",
			model:          "dummy",
			expectedOutput: "This is a dummy backend response",
			expectError:    false,
		},
		{
			name:           "empty input",
			input:          "",
			backend:        "dummy",
			model:          "dummy",
			expectedOutput: "This is a dummy backend response",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			cfg := &Config{
				Backend: tt.backend,
				Model:   tt.model,
			}

			model, err := InitializeModel(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize model: %v", err)
			}

			service, err := NewCompletionService(cfg, model,
				WithStdout(&stdout),
				WithStderr(&stderr),
				WithDisableHistory(true),
			)
			if err != nil {
				t.Fatalf("Failed to create completion service: %v", err)
			}

			opts := RunOptions{
				Config:       cfg,
				InputStrings: []string{tt.input},
				StreamOutput: false,
				ShowSpinner:  false,
				Stdout:       &stdout,
				Stderr:       &stderr,
				Stdin:        strings.NewReader(""),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = service.Run(ctx, opts)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				output := stdout.String()
				if !strings.Contains(output, tt.expectedOutput) {
					t.Errorf("Expected output to contain %q, got %q", tt.expectedOutput, output)
				}
			}
		})
	}
}

// TestIntegration_HistoryPersistence tests history file creation and loading
func TestIntegration_HistoryPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "test_history.yaml")

	// Create a test history
	testHistory := history{
		Backend: "dummy",
		Model:   "dummy",
		Messages: []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, "Hello"),
			llms.TextParts(llms.ChatMessageTypeAI, "Hi there!"),
		},
	}

	// Save the history
	historyData, err := yaml.Marshal(testHistory)
	if err != nil {
		t.Fatalf("Failed to marshal test history: %v", err)
	}

	if err := AtomicWriteFile(historyFile, historyData, 0644); err != nil {
		t.Fatalf("Failed to write history file: %v", err)
	}

	// Test loading history
	var stdout, stderr bytes.Buffer
	cfg := &Config{
		Backend: "dummy",
		Model:   "dummy",
	}

	model, err := InitializeModel(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}

	service, err := NewCompletionService(cfg, model,
		WithStdout(&stdout),
		WithStderr(&stderr),
	)
	if err != nil {
		t.Fatalf("Failed to create completion service: %v", err)
	}

	opts := RunOptions{
		Config:       cfg,
		HistoryIn:    historyFile,
		InputStrings: []string{"Continue conversation"},
		StreamOutput: false,
		ShowSpinner:  false,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Stdin:        strings.NewReader(""),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = service.Run(ctx, opts)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify history was loaded (service should have the previous messages)
	if len(service.payload.Messages) < 2 {
		t.Errorf("Expected at least 2 messages from loaded history, got %d", len(service.payload.Messages))
	}
}

// TestIntegration_ConfigurationLoading tests config loading from various sources
func TestIntegration_ConfigurationLoading(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name            string
		configFile      string
		envVars         map[string]string
		flags           []string
		expectedBackend string
		expectedModel   string
	}{
		{
			name: "config file only",
			configFile: `
backend: anthropic
model: claude-3-sonnet-20240229
temperature: 0.7
maxTokens: 4000
`,
			expectedBackend: "anthropic",
			expectedModel:   "claude-3-sonnet-20240229",
		},
		{
			name: "env vars override config",
			envVars: map[string]string{
				"CGPT_BACKEND": "openai",
				"CGPT_MODEL":   "gpt-4-turbo-preview", // Use the full model name to avoid alias expansion
			},
			configFile: `
backend: anthropic
model: claude-3-sonnet-20240229
`,
			expectedBackend: "openai",
			expectedModel:   "gpt-4-turbo-preview",
		},
		{
			name:  "flags override all",
			flags: []string{"--backend", "dummy", "--model", "test-model"},
			envVars: map[string]string{
				"CGPT_BACKEND": "openai",
			},
			configFile: `
backend: anthropic
model: claude-3-sonnet-20240229
`,
			expectedBackend: "dummy",
			expectedModel:   "test-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up config file
			configPath := filepath.Join(tmpDir, "config.yaml")
			if tt.configFile != "" {
				if err := AtomicWriteFile(configPath, []byte(tt.configFile), 0644); err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			// Set up environment variables
			oldEnv := make(map[string]string)
			for key, value := range tt.envVars {
				oldEnv[key] = os.Getenv(key)
				os.Setenv(key, value)
			}
			defer func() {
				for key, oldValue := range oldEnv {
					if oldValue == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, oldValue)
					}
				}
			}()

			// Set up flags
			flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
			var opts RunOptions
			opts.Config = &Config{}
			defineFlags(flagSet, &opts)

			args := append([]string{"--config", configPath}, tt.flags...)
			if err := flagSet.Parse(args); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			var stderr bytes.Buffer
			cfg, err := LoadConfig(configPath, &stderr, flagSet)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			if cfg.Backend != tt.expectedBackend {
				t.Errorf("Expected backend %q, got %q", tt.expectedBackend, cfg.Backend)
			}
			if cfg.Model != tt.expectedModel {
				t.Errorf("Expected model %q, got %q", tt.expectedModel, cfg.Model)
			}
		})
	}
}

// TestIntegration_ErrorHandling tests various error scenarios
func TestIntegration_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*testing.T) RunOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "invalid backend",
			setupFunc: func(t *testing.T) RunOptions {
				return RunOptions{
					Config:       &Config{Backend: "nonexistent", Model: "dummy"},
					InputStrings: []string{"test"},
					StreamOutput: false,
					ShowSpinner:  false,
				}
			},
			expectError: true,
			errorMsg:    "unknown backend",
		},
		{
			name: "nonexistent input file",
			setupFunc: func(t *testing.T) RunOptions {
				return RunOptions{
					Config:       &Config{Backend: "dummy", Model: "dummy"},
					InputFiles:   []string{"/nonexistent/file.txt"},
					StreamOutput: false,
					ShowSpinner:  false,
				}
			},
			expectError: true,
			errorMsg:    "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			opts := tt.setupFunc(t)
			opts.Stdout = &stdout
			opts.Stderr = &stderr
			opts.Stdin = strings.NewReader("")

			model, err := InitializeModel(opts.Config)
			if err != nil && tt.expectError && strings.Contains(err.Error(), tt.errorMsg) {
				// This is expected, the error happened during model initialization
				return
			} else if err != nil {
				t.Fatalf("Failed to initialize model: %v", err)
			}

			service, err := NewCompletionService(opts.Config, model,
				WithStdout(&stdout),
				WithStderr(&stderr),
			)
			if err != nil {
				t.Fatalf("Failed to create completion service: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = service.Run(ctx, opts)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, but got no error", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestIntegration_ContextCancellation tests proper handling of context cancellation
func TestIntegration_ContextCancellation(t *testing.T) {
	var stdout, stderr bytes.Buffer

	cfg := &Config{
		Backend: "dummy",
		Model:   "dummy",
	}

	model, err := InitializeModel(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}

	service, err := NewCompletionService(cfg, model,
		WithStdout(&stdout),
		WithStderr(&stderr),
		WithDisableHistory(true),
	)
	if err != nil {
		t.Fatalf("Failed to create completion service: %v", err)
	}

	opts := RunOptions{
		Config:       cfg,
		InputStrings: []string{"Test input"},
		StreamOutput: false,
		ShowSpinner:  false,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Stdin:        strings.NewReader(""),
	}

	// Create context that will be cancelled immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = service.Run(ctx, opts)

	// Should handle cancellation gracefully
	if err != nil && err != context.Canceled {
		// Some cancellation errors are expected, but not all errors
		t.Logf("Got error (may be expected): %v", err)
	}
}

// TestIntegration_MultipleBackends tests switching between different backends
func TestIntegration_MultipleBackends(t *testing.T) {
	backends := []struct {
		name    string
		backend string
		model   string
	}{
		{"dummy", "dummy", "dummy"},
	}

	for _, b := range backends {
		t.Run(b.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			cfg := &Config{
				Backend: b.backend,
				Model:   b.model,
			}

			model, err := InitializeModel(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize %s backend: %v", b.backend, err)
			}

			service, err := NewCompletionService(cfg, model,
				WithStdout(&stdout),
				WithStderr(&stderr),
				WithDisableHistory(true),
			)
			if err != nil {
				t.Fatalf("Failed to create completion service for %s: %v", b.backend, err)
			}

			opts := RunOptions{
				Config:       cfg,
				InputStrings: []string{"Test backend switching"},
				StreamOutput: false,
				ShowSpinner:  false,
				Stdout:       &stdout,
				Stderr:       &stderr,
				Stdin:        strings.NewReader(""),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = service.Run(ctx, opts)
			if err != nil {
				t.Errorf("Failed to run completion with %s backend: %v", b.backend, err)
			}

			// Verify we got some output
			if stdout.Len() == 0 {
				t.Errorf("Expected output from %s backend, got none", b.backend)
			}
		})
	}
}

// TestIntegration_InteractiveMode tests non-continuous mode simulation
func TestIntegration_InteractiveMode(t *testing.T) {
	// Note: True interactive mode with continuous flag would require complex
	// terminal simulation. This test focuses on the non-continuous aspects
	// that can be reliably tested.

	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "interactive_test.yaml")

	var stdout, stderr bytes.Buffer
	cfg := &Config{
		Backend: "dummy",
		Model:   "dummy",
	}

	model, err := InitializeModel(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}

	service, err := NewCompletionService(cfg, model,
		WithStdout(&stdout),
		WithStderr(&stderr),
	)
	if err != nil {
		t.Fatalf("Failed to create completion service: %v", err)
	}

	// Simulate interactive-like behavior with multiple rounds
	tests := []struct {
		name  string
		input string
	}{
		{"first_interaction", "Hello, this is the first message."},
		{"second_interaction", "This is a follow-up message."},
		{"third_interaction", "And this is the third message in our conversation."},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()

			opts := RunOptions{
				Config:       cfg,
				InputStrings: []string{tt.input},
				History:      historyFile, // Persistent history across interactions
				StreamOutput: false,
				ShowSpinner:  false,
				Stdout:       &stdout,
				Stderr:       &stderr,
				Stdin:        strings.NewReader(""),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := service.Run(ctx, opts)
			if err != nil {
				t.Errorf("Interactive simulation failed: %v", err)
			}

			// Verify output was generated
			if stdout.Len() == 0 {
				t.Errorf("Expected output from interaction %d, got none", i+1)
			}

			// After first interaction, verify history file exists and grows
			if i == 0 {
				if _, err := os.Stat(historyFile); os.IsNotExist(err) {
					t.Errorf("Expected history file to be created after first interaction")
				}
			}
		})
	}

	// Verify accumulated history
	if _, err := os.Stat(historyFile); err == nil {
		historyData, err := os.ReadFile(historyFile)
		if err != nil {
			t.Fatalf("Failed to read history file: %v", err)
		}

		var h history
		if err := yaml.Unmarshal(historyData, &h); err != nil {
			t.Fatalf("Failed to unmarshal history: %v", err)
		}

		// Should have accumulated multiple message exchanges
		// Each test adds 2 messages (human + assistant)
		expectedMinMessages := len(tests) * 2
		if len(h.Messages) < expectedMinMessages {
			t.Errorf("Expected at least %d messages in accumulated history, got %d",
				expectedMinMessages, len(h.Messages))
		}
	}
}

// TestIntegration_StreamingOutput tests streaming vs non-streaming output modes
func TestIntegration_StreamingOutput(t *testing.T) {
	tests := []struct {
		name      string
		streaming bool
	}{
		{"streaming", true},
		{"non-streaming", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			cfg := &Config{
				Backend: "dummy",
				Model:   "dummy",
			}

			model, err := InitializeModel(cfg)
			if err != nil {
				t.Fatalf("Failed to initialize model: %v", err)
			}

			service, err := NewCompletionService(cfg, model,
				WithStdout(&stdout),
				WithStderr(&stderr),
				WithDisableHistory(true),
			)
			if err != nil {
				t.Fatalf("Failed to create completion service: %v", err)
			}

			opts := RunOptions{
				Config:       cfg,
				InputStrings: []string{"Test streaming"},
				StreamOutput: tt.streaming,
				ShowSpinner:  false,
				Stdout:       &stdout,
				Stderr:       &stderr,
				Stdin:        strings.NewReader(""),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err = service.Run(ctx, opts)
			if err != nil {
				t.Errorf("Failed to run completion: %v", err)
			}

			// Both modes should produce output
			if stdout.Len() == 0 {
				t.Errorf("Expected output in %s mode, got none", tt.name)
			}
		})
	}
}

// TestIntegration_UsageTracking tests usage/cost tracking functionality
func TestIntegration_UsageTracking(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "usage_test.yaml")

	var stdout, stderr bytes.Buffer
	cfg := &Config{
		Backend:   "dummy",
		Model:     "dummy",
		ShowUsage: true,
	}

	model, err := InitializeModel(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize model: %v", err)
	}

	service, err := NewCompletionService(cfg, model,
		WithStdout(&stdout),
		WithStderr(&stderr),
	)
	if err != nil {
		t.Fatalf("Failed to create completion service: %v", err)
	}

	opts := RunOptions{
		Config:       cfg,
		InputStrings: []string{"Test usage tracking"},
		History:      historyFile,
		StreamOutput: false,
		ShowSpinner:  false,
		Stdout:       &stdout,
		Stderr:       &stderr,
		Stdin:        strings.NewReader(""),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = service.Run(ctx, opts)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that history file was created with usage info
	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		t.Errorf("Expected history file to be created at %s", historyFile)
		return
	}

	// Read and verify the history contains usage information
	historyData, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatalf("Failed to read history file: %v", err)
	}

	var h history
	if err := yaml.Unmarshal(historyData, &h); err != nil {
		t.Fatalf("Failed to unmarshal history: %v", err)
	}

	if h.Metadata == nil || h.Metadata.UsageInfo == nil {
		t.Errorf("Expected usage info in history metadata")
	}
}

// BenchmarkIntegration_Completion benchmarks completion performance
func BenchmarkIntegration_Completion(b *testing.B) {
	cfg := &Config{
		Backend: "dummy",
		Model:   "dummy",
	}

	model, err := InitializeModel(cfg)
	if err != nil {
		b.Fatalf("Failed to initialize model: %v", err)
	}

	service, err := NewCompletionService(cfg, model,
		WithStdout(io.Discard),
		WithStderr(io.Discard),
		WithDisableHistory(true),
	)
	if err != nil {
		b.Fatalf("Failed to create completion service: %v", err)
	}

	opts := RunOptions{
		Config:       cfg,
		InputStrings: []string{"Benchmark test input"},
		StreamOutput: false,
		ShowSpinner:  false,
		Stdout:       io.Discard,
		Stderr:       io.Discard,
		Stdin:        strings.NewReader(""),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := service.Run(ctx, opts)
		cancel()
		if err != nil {
			b.Errorf("Completion failed: %v", err)
		}
	}
}

// BenchmarkIntegration_HistoryOperations benchmarks history loading and saving
func BenchmarkIntegration_HistoryOperations(b *testing.B) {
	tmpDir := b.TempDir()

	// Create test history
	testHistory := history{
		Backend: "dummy",
		Model:   "dummy",
		Messages: []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, "Test message 1"),
			llms.TextParts(llms.ChatMessageTypeAI, "Test response 1"),
			llms.TextParts(llms.ChatMessageTypeHuman, "Test message 2"),
			llms.TextParts(llms.ChatMessageTypeAI, "Test response 2"),
		},
	}

	historyData, err := yaml.Marshal(testHistory)
	if err != nil {
		b.Fatalf("Failed to marshal test history: %v", err)
	}

	b.ResetTimer()

	b.Run("HistoryLoad", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			historyFile := filepath.Join(tmpDir, fmt.Sprintf("bench_history_%d.yaml", i))
			if err := AtomicWriteFile(historyFile, historyData, 0644); err != nil {
				b.Errorf("Failed to write history file: %v", err)
			}

			cfg := &Config{Backend: "dummy", Model: "dummy"}
			model, err := InitializeModel(cfg)
			if err != nil {
				b.Errorf("Failed to initialize model: %v", err)
			}

			service, err := NewCompletionService(cfg, model,
				WithStdout(io.Discard),
				WithStderr(io.Discard),
			)
			if err != nil {
				b.Errorf("Failed to create service: %v", err)
			}

			// Load history
			file, err := os.Open(historyFile)
			if err != nil {
				b.Errorf("Failed to open history file: %v", err)
			}
			service.historyIn = file
			if err := service.loadHistory(); err != nil {
				b.Errorf("Failed to load history: %v", err)
			}
			file.Close()
		}
	})

	b.Run("HistorySave", func(b *testing.B) {
		cfg := &Config{Backend: "dummy", Model: "dummy"}
		model, err := InitializeModel(cfg)
		if err != nil {
			b.Fatalf("Failed to initialize model: %v", err)
		}

		service, err := NewCompletionService(cfg, model,
			WithStdout(io.Discard),
			WithStderr(io.Discard),
		)
		if err != nil {
			b.Fatalf("Failed to create service: %v", err)
		}

		// Set up test messages
		service.payload.Messages = testHistory.Messages

		for i := 0; i < b.N; i++ {
			historyFile := filepath.Join(tmpDir, fmt.Sprintf("bench_save_%d.yaml", i))
			service.historyOutFile = historyFile
			if err := service.saveHistory(); err != nil {
				b.Errorf("Failed to save history: %v", err)
			}
		}
	})
}

// BenchmarkIntegration_ConfigurationParsing benchmarks config loading performance
func BenchmarkIntegration_ConfigurationParsing(b *testing.B) {
	tmpDir := b.TempDir()

	// Create a comprehensive config file
	configContent := `
backend: dummy
model: dummy
temperature: 0.7
maxTokens: 4000
systemPrompt: "You are a helpful assistant"
promptCaching: true
showUsage: true
showReasoning: false
thinkingMode: medium
thinkingBudget: 1024
interleavedThinking: false
maxRetries: 3
retryDelay: 1s
disableRetry: false
completionTimeout: 120s
`

	configFile := filepath.Join(tmpDir, "benchmark_config.yaml")
	if err := AtomicWriteFile(configFile, []byte(configContent), 0644); err != nil {
		b.Fatalf("Failed to write config file: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		flagSet := pflag.NewFlagSet("benchmark", pflag.ContinueOnError)
		var opts RunOptions
		opts.Config = &Config{}
		defineFlags(flagSet, &opts)

		// Parse with config file
		args := []string{"--config", configFile}
		if err := flagSet.Parse(args); err != nil {
			b.Errorf("Failed to parse flags: %v", err)
		}

		_, err := LoadConfig(configFile, io.Discard, flagSet)
		if err != nil {
			b.Errorf("Failed to load config: %v", err)
		}
	}
}

// BenchmarkIntegration_StreamingVsNonStreaming compares streaming vs non-streaming performance
func BenchmarkIntegration_StreamingVsNonStreaming(b *testing.B) {
	cfg := &Config{
		Backend: "dummy",
		Model:   "dummy",
	}

	model, err := InitializeModel(cfg)
	if err != nil {
		b.Fatalf("Failed to initialize model: %v", err)
	}

	b.Run("Streaming", func(b *testing.B) {
		service, err := NewCompletionService(cfg, model,
			WithStdout(io.Discard),
			WithStderr(io.Discard),
			WithDisableHistory(true),
		)
		if err != nil {
			b.Fatalf("Failed to create completion service: %v", err)
		}

		opts := RunOptions{
			Config:       cfg,
			InputStrings: []string{"Streaming benchmark test"},
			StreamOutput: true,
			ShowSpinner:  false,
			Stdout:       io.Discard,
			Stderr:       io.Discard,
			Stdin:        strings.NewReader(""),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := service.Run(ctx, opts)
			cancel()
			if err != nil {
				b.Errorf("Streaming completion failed: %v", err)
			}
		}
	})

	b.Run("NonStreaming", func(b *testing.B) {
		service, err := NewCompletionService(cfg, model,
			WithStdout(io.Discard),
			WithStderr(io.Discard),
			WithDisableHistory(true),
		)
		if err != nil {
			b.Fatalf("Failed to create completion service: %v", err)
		}

		opts := RunOptions{
			Config:       cfg,
			InputStrings: []string{"Non-streaming benchmark test"},
			StreamOutput: false,
			ShowSpinner:  false,
			Stdout:       io.Discard,
			Stderr:       io.Discard,
			Stdin:        strings.NewReader(""),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := service.Run(ctx, opts)
			cancel()
			if err != nil {
				b.Errorf("Non-streaming completion failed: %v", err)
			}
		}
	})
}

// defineFlags defines command line flags for testing (simplified version)
func defineFlags(fs *pflag.FlagSet, opts *RunOptions) {
	fs.StringVarP(&opts.Config.Backend, "backend", "b", "anthropic", "The backend to use")
	fs.StringVarP(&opts.Config.Model, "model", "m", "claude-sonnet-4-20250514", "The model to use")
	fs.StringVarP(&opts.Config.SystemPrompt, "system-prompt", "s", "", "System prompt to use")
	fs.IntVarP(&opts.Config.MaxTokens, "max-tokens", "t", 0, "Maximum tokens to generate")
	fs.Float64VarP(&opts.Config.Temperature, "temperature", "T", 0.05, "Temperature for sampling")
	fs.BoolVar(&opts.Config.ShowUsage, "usage", false, "Show token usage and cost estimates")
	fs.IntVar(&opts.Config.MaxRetries, "max-retries", 3, "Maximum number of retry attempts")
	fs.DurationVar(&opts.Config.RetryDelay, "retry-delay", time.Second, "Initial retry delay")
	fs.BoolVar(&opts.Config.DisableRetry, "disable-retry", false, "Disable retry attempts")
	fs.StringVar(&opts.ConfigPath, "config", "config.yaml", "Path to the configuration file")
}
