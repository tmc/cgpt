package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
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

func Test(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		backend string
		model   string
		args    []string
	}{
		{
			name:    "basic dummy",
			backend: "dummy",
			model:   "dummy-model",
		},
		{
			name:    "dummy with debug",
			backend: "dummy",
			model:   "dummy-model",
			args:    []string{"--debug"},
		},
		{
			name:    "dummy flag test",
			backend: "dummy",
			model:   "dummy-model",
			args: []string{
				`-s`, `you are a yq expert`,
				`-i`, `how can i force "pipe" mode in yq`,
			},
		},
		{
			name:    "ollama model",
			backend: "ollama",
			model:   "llama3.2:1b",
			args:    []string{"--prefill=yo"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testName := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(t.Name(), "_"))
			testInputFile := filepath.Join("testdata", fmt.Sprintf("%s.txtar", testName))

			var (
				inBuf  = new(bytes.Buffer)
				outBuf = new(bytes.Buffer)
				errBuf = new(bytes.Buffer)
			)

			txtarComment, files, err := readTxtarFile(t, testInputFile)
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("Failed to read input file: %v", err)
			}
			if errors.Is(err, fs.ErrNotExist) {
				files = make(map[string][]byte)
				if *update {
					updateGoldenFile(t, testInputFile, "what is your name?", nil, nil, nil, nil)
					t.Skip("Skipping test as golden file created")
				}
			}

			args := []string{"cgpt-test", fmt.Sprintf("--backend=%s", tc.backend), fmt.Sprintf("--model=%s", tc.model)}
			args = append(args, tc.args...)
			opts, fs, err := initFlags(args, inBuf)
			if err != nil {
				t.Fatalf("initFlags: %v", err)
			}
			opts.Stderr = errBuf
			opts.Stdout = outBuf
			inBuf.WriteString(txtarComment)

			runTest(t, context.Background(), opts, fs, newTestLogger(t))
			if *update {
				updateGoldenFile(t, testInputFile, txtarComment, files, outBuf.Bytes(), errBuf.Bytes(), files["http_payload"])
				t.SkipNow()
			}
			compareOutput(t, files["stdout"], files["stderr"], outBuf.Bytes(), errBuf.Bytes(), files["http_payload"])
		})
	}
}

func runTest(t *testing.T, ctx context.Context, opts cgpt.RunOptions, fs *pflag.FlagSet, logger *zap.SugaredLogger) {
	t.Helper()
	fileCfg, err := cgpt.LoadConfig(opts.ConfigPath, opts.Stderr, fs)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	opts.Config = fileCfg

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
	if err := s.Run(ctx, opts); err != nil {
		t.Fatalf("failed to run completion service: %v", err)
	}
}

func shellSplit(t *testing.T, cmdString string) ([]string, error) {
	t.Helper()
	cmd := exec.Command("sh", "-c", fmt.Sprintf("printf '%%s\\0' %s", cmdString))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("shell split failed: %w", err)
	}
	return strings.Split(strings.TrimRight(string(output), "\x00"), "\x00"), nil
}

func TestShellQuoting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{
			name:    "basic quotes",
			command: `cgpt -s "system prompt" -i "basic input"`,
			wantErr: false,
		},
		{
			name:    "unescaped quotes",
			command: `cgpt -s "expert" -i "how can i force "pipe" mode in yq"`,
			wantErr: false, // shell treats as literals, merges parts
		},
		{
			name:    "single quotes",
			command: `cgpt -s 'expert' -i 'how can i force "pipe" mode in yq'`,
			wantErr: false,
		},
		{
			name:    "unterminated quote",
			command: `cgpt -s "unterminated`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := shellSplit(t, tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("shellSplit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				t.Logf("Shell-split arguments: %#v", args)
				_, _, err = initFlags(args, strings.NewReader(""))
				if err != nil {
					t.Errorf("unexpected initFlags error: %v", err)
				}
			}
		})
	}
}

func newTestLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller(), zap.AddCallerSkip(1)))
	t.Cleanup(func() { _ = logger.Sync() })
	return logger.Sugar()
}

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

func updateGoldenFile(t *testing.T, path, comment string, files map[string][]byte, stdout, stderr, httpPayload []byte) {
	t.Helper()
	ar := &txtar.Archive{Files: []txtar.File{}}
	for _, k := range slices.Sorted(maps.Keys(files)) {
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

func compareBytes(t *testing.T, name string, want, got []byte) {
	t.Helper()
	wantStr := strings.TrimRight(string(want), "\n") + "\n"
	gotStr := strings.TrimRight(string(got), "\n") + "\n"
	if diff := cmp.Diff(wantStr, gotStr); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", name, diff)
	}
}
