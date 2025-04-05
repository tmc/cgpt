package options

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestBackendDefaultModel(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("CGPT_BACKEND", "dummy")

	def := Config{Stream: true, MaxTokens: 4096, Temperature: 0.05}
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
			want:     Config{Backend: "dummy", Model: "dummy", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature},
			wantLogs: "cgpt: using default model for dummy backend: dummy",
		},
		{
			name:  "flag backend but explicit model flag preserved",
			flags: []string{"--backend=dummy", "--model=dummy-custom"},
			want:  Config{Backend: "dummy", Model: "dummy-custom", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature},
		},
		{
			name:  "flag backend but env model preserved",
			flags: []string{"--backend=dummy"},
			env:   map[string]string{"CGPT_MODEL": "dummy-custom"},
			want:  Config{Backend: "dummy", Model: "dummy-custom", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature},
		},
		{
			name:       "flag backend but config model preserved",
			configYAML: "model: dummy-custom",
			flags:      []string{"--backend=dummy"},
			want:       Config{Backend: "dummy", Model: "dummy-custom", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature},
		},
		{
			name:     "env backend uses its default model",
			env:      map[string]string{"CGPT_BACKEND": "dummy"},
			want:     Config{Backend: "dummy", Model: "dummy", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature},
			wantLogs: "cgpt: using default model for dummy backend: dummy",
		},
		{
			name:       "config backend uses its default model",
			configYAML: "backend: dummy",
			want:       Config{Backend: "dummy", Model: "dummy", Stream: def.Stream, MaxTokens: def.MaxTokens, Temperature: def.Temperature},
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
