package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// readTxtarFile reads a txtar file.
func readTxtarFile(t *testing.T, path string) (string, map[string][]byte, error) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("Failed to read txtar file: %w", err)
	}
	ar := txtar.Parse(content)
	result := make(map[string][]byte)
	for _, f := range ar.Files {
		result[f.Name] = f.Data
	}
	return string(ar.Comment), result, nil
}

func Test(t *testing.T) {
	t.Setenv("CGPT_TEMPERATURE", "0.0")
	testCases := []struct {
		name    string
		backend string
		model   string
		args    []string
		env     map[string]string
	}{
		{name: "basic dummy", backend: "dummy", model: "dummy-model"},
		{name: "dummy with debug", backend: "dummy", model: "dummy-model", args: []string{"--debug"}},
		{name: "dummy with stop sequence", backend: "dummy", model: "dummy-model", args: []string{"--prefill=```test", "--prefill-echo=false"}},
		{name: "dummy with vimrc stop sequence", backend: "dummy", model: "dummy-model", args: []string{"--prefill=```vimrc", "--prefill-echo=false", "--system-prompt=you are a vimrc expert"}},
		{name: "ollama llama3.2", backend: "ollama", model: "llama3.2:1b"},
		{name: "ollama llama3.2 prefill", backend: "ollama", model: "llama3.2:1b", args: []string{"--prefill=yo"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testName := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(t.Name(), "_"))
			testInputFile := filepath.Join("testdata", fmt.Sprintf("%s.txtar", testName))

			logger := newTestLogger(t)
			var (
				inBuf  = new(bytes.Buffer)
				outBuf = new(bytes.Buffer)
				errBuf = new(bytes.Buffer)
			)

			// Read input from txtar file
			txtarComment, files, err := readTxtarFile(t, testInputFile)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					files = make(map[string][]byte)
					if *update {
						t.Logf("Creating new golden file: %s", testInputFile)
						updateGoldenFile(t, testInputFile, "what is your name?", nil, nil, nil, nil)
						t.Skip("Skipping test as golden file created")
					}
				} else {
					t.Fatalf("Failed to read input file: %v", err)
				}
			}

			args := []string{"cgpt-test", fmt.Sprintf("--backend=%s", tc.backend), fmt.Sprintf("--model=%s", tc.model)}
			args = append(args, tc.args...)
			//args := append(tc.args, "--debug")
			t.Log("Input:", txtarComment)
			t.Logf("Args: %q", args)
			opts, fs, err := initFlags(args, inBuf)
			opts.Stderr = errBuf
			opts.Stdout = outBuf
			inBuf.WriteString(txtarComment)

			ctx := context.Background()
			runTest(t, ctx, opts, fs, logger)
			if *update {
				updateGoldenFile(t, testInputFile, txtarComment, files, outBuf.Bytes(), errBuf.Bytes(), files["http_payload"])
				t.SkipNow()
			}
			if outBuf.Len() == 0 {
				t.Error("No output from test")
			}
			compareOutput(t, files["stdout"], files["stderr"], outBuf.Bytes(), errBuf.Bytes(), files["http_payload"])
		})
	}
}

func updateGoldenFile(t *testing.T, path, comment string, files map[string][]byte, stdout, stderr, httpPayload []byte) {
	t.Helper()
	ar := &txtar.Archive{
		Files: []txtar.File{},
	}
	// Get sorted keys without using slices.Sorted and maps.Keys
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		if k == "stdout" || k == "stderr" || k == "http_payload" {
			continue
		}
		ar.Files = append(ar.Files, txtar.File{Name: k, Data: files[k]})
	}
	if comment != "" {
		ar.Comment = []byte(comment)
	}
	if stdout != nil {
		ar.Files = append(ar.Files, txtar.File{Name: "stdout", Data: stdout})
	}
	if stderr != nil {
		ar.Files = append(ar.Files, txtar.File{Name: "stderr", Data: stderr})
	}
	if httpPayload != nil {
		ar.Files = append(ar.Files, txtar.File{Name: "http_payload", Data: httpPayload})
	}
	t.Log("Updating golden file:", path)
	t.Log("Contents:", string(txtar.Format(ar)))
	if err := os.WriteFile(path, txtar.Format(ar), 0644); err != nil {
		t.Fatalf("Failed to update golden file: %v", err)
	}
}

func compareOutput(t *testing.T, expectedStdout, expectedStderr, stdout, stderr, httpPayload []byte) {
	t.Helper()
	compareBytes(t, "stdout", expectedStdout, stdout)
	compareBytes(t, "stderr", expectedStderr, stderr)
	if httpPayload != nil {
		compareBytes(t, "http_payload", expectedStdout, httpPayload)
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

func runTest(t *testing.T, ctx context.Context, opts cgpt.RunOptions, fs *pflag.FlagSet, logger *zap.SugaredLogger) {
	t.Helper()
	fileCfg, err := cgpt.LoadConfig(opts.ConfigPath, opts.Stderr, fs)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	opts.Config = fileCfg

	t.Logf("Options: %+v\n", opts)
	t.Logf("Config: %+v\n", opts.Config)

	// TODO: add http client for debug/replay
	// modelOpts := []cgpt.ModelOption{
	// 	cgpt.WithHTTPClient(httputil.DebugHTTPClient),
	// }
	model, err := cgpt.InitializeModel(opts.Config)
	if err != nil {
		t.Fatalf("failed to initialize model: %v", err)
	}

	s, err := cgpt.NewCompletionService(opts.Config, model,
		cgpt.WithStdout(opts.Stdout),
		cgpt.WithStderr(opts.Stderr),
		cgpt.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("failed to create completion service: %v", err)
	}
	err = s.Run(ctx, opts)
	if err != nil {
		t.Fatalf("failed to run completion service: %v", err)
	}
}
