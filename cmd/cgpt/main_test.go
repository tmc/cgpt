package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/pflag"
	"github.com/tmc/cgpt"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/tools/txtar"
)

var update = flag.Bool("update", false, "update golden files")

func newTestLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	// set logger based on the test level, and add to stack depth
	level := zap.DebugLevel
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zap.ErrorLevel),
		zap.IncreaseLevel(level),
	))
	t.Cleanup(func() { _ = logger.Sync() })
	return logger.Sugar()
}

// readTxtarFile reads a txtar file and returns a map of its contents
func readTxtarFile(t *testing.T, path string) map[string][]byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read txtar file: %v", err)
	}
	ar := txtar.Parse(content)
	result := make(map[string][]byte)
	for _, f := range ar.Files {
		result[f.Name] = f.Data
	}
	return result
}

// TestRunWithBackends runs tests with different backends and input files
func TestRunWithBackends(t *testing.T) {
	testCases := []struct {
		name    string
		backend string
		model   string
		input   string
		args    []string
		envVars map[string]string
		skip    bool
		skipMsg string

		zapOpts []zap.Option
	}{
		{name: "basic dummy", backend: "dummy", model: "dummy-model", input: "testdata/basic_input.txt"},
		{name: "dummy with debug", backend: "dummy", model: "dummy-model", input: "testdata/basic_input.txt", zapOpts: []zap.Option{zap.IncreaseLevel(zap.DebugLevel)}},
		{name: "openai gpt-3.5", backend: "openai", model: "gpt-3.5-turbo", input: "testdata/basic_input.txt"},
		{name: "openai gpt-4 complex", backend: "openai", model: "gpt-4", input: "testdata/complex_input.txt", envVars: map[string]string{"CGPT_OPENAI_API_KEY": "test-api-key"}},
		{name: "anthropic claude", backend: "anthropic", model: "claude-v1", input: "testdata/basic_input.txt", skip: true, skipMsg: "Anthropic API not available in CI"},
		{name: "ollama llama2 temp", backend: "ollama", model: "llama2", input: "testdata/basic_input.txt", args: []string{"--temperature", "0.7"}},
		{name: "dummy with history", backend: "dummy", model: "dummy-model", input: "testdata/history_input.txt", args: []string{"--history-in", "testdata/history.yaml"}},
	}

	for _, tc := range testCases {
		testName := strings.ReplaceAll(fmt.Sprintf("%s_%s_%s", tc.name, tc.backend, tc.model), "/", "_")
		t.Run(testName, func(t *testing.T) {
			if tc.skip {
				t.Skip(tc.skipMsg)
			}

			logger := newTestLogger(t)
			var outBuf, errBuf bytes.Buffer

			// Read input from txtar file
			inputContent := readTxtarFile(t, tc.input)
			input := bytes.NewReader(inputContent["input"])

			opts := cgpt.RunOptions{
				Config: &cgpt.Config{},
				Stdin:  input,
				Stdout: &outBuf,
				Stderr: &errBuf,
			}
			fs := pflag.NewFlagSet("cgpt-test", pflag.ContinueOnError)
			defineFlags(fs, &opts)

			args := append([]string{"cgpt-test", "-b", tc.backend, "--model", tc.model}, tc.args...)
			if err := fs.Parse(args); err != nil {
				t.Fatalf("Failed to parse flags: %v", err)
			}

			// Set environment variables
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			err := runTest(context.Background(), opts, fs, logger)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			goldenPath := filepath.Join("testdata", fmt.Sprintf("%s_%s.txtar", tc.backend, tc.model))
			if *update {
				updateGoldenFile(t, goldenPath, outBuf.Bytes(), errBuf.Bytes(), inputContent["http_payload"])
			}

			expectedContent := readTxtarFile(t, goldenPath)
			compareOutput(t, expectedContent, outBuf.Bytes(), errBuf.Bytes(), inputContent["http_payload"])
		})
	}
}

// updateGoldenFile updates the golden txtar file with the current test output
func updateGoldenFile(t *testing.T, path string, stdout, stderr, httpPayload []byte) {
	t.Helper()
	ar := &txtar.Archive{
		Files: []txtar.File{
			{Name: "stdout", Data: stdout},
			{Name: "stderr", Data: stderr},
		},
	}
	if httpPayload != nil {
		ar.Files = append(ar.Files, txtar.File{Name: "http_payload", Data: httpPayload})
	}
	if err := ioutil.WriteFile(path, txtar.Format(ar), 0644); err != nil {
		t.Fatalf("Failed to update golden file: %v", err)
	}
}

// compareOutput compares the test output with the expected output from the golden file
func compareOutput(t *testing.T, expected map[string][]byte, stdout, stderr, httpPayload []byte) {
	t.Helper()
	compareBytes(t, "stdout", expected["stdout"], stdout)
	compareBytes(t, "stderr", expected["stderr"], stderr)
	if httpPayload != nil {
		compareBytes(t, "http_payload", expected["http_payload"], httpPayload)
	}
}

// compareBytes compares two byte slices and reports any differences
func compareBytes(t *testing.T, name string, want, got []byte) {
	t.Helper()
	wantStr := strings.TrimRight(string(want), "\n") + "\n"
	gotStr := strings.TrimRight(string(got), "\n") + "\n"
	if diff := cmp.Diff(wantStr, gotStr); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", name, diff)
	}
}

func runTest(ctx context.Context, opts cgpt.RunOptions, fs *pflag.FlagSet, logger *zap.SugaredLogger) error {
	fileCfg, err := cgpt.LoadConfig(opts.ConfigPath, opts.Stderr, fs)
	if err != nil {
		return err
	}

	cfg := cgpt.MergeConfigs(*fileCfg, *opts.Config)
	opts.Config = &cfg

	model, err := cgpt.InitializeModel(opts.Config)
	if err != nil {
		return err
	}

	s, err := cgpt.NewCompletionService(opts.Config, model,
		cgpt.WithStdout(opts.Stdout),
		cgpt.WithStderr(opts.Stderr),
		cgpt.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("failed to create completion service: %w", err)
	}
	return s.Run(ctx, opts)
}
