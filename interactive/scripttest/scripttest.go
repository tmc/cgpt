// Package scripttest provides utilities for testing terminal UX with shell scripts
package scripttest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tmc/cgpt/internal/rr"
)

// ScriptTest represents a test that runs a shell script
type ScriptTest struct {
	T             *testing.T
	ScriptPath    string
	Env           []string
	Timeout       time.Duration
	HTTPTraceFile string
	httpRR        *rr.RecordReplay
}

// New creates a new ScriptTest
func New(t *testing.T, scriptContent string) (*ScriptTest, error) {
	// Create a temporary directory for the script
	tmpDir, err := os.MkdirTemp("", "scripttest-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Create the script file
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}

	// Create the test
	test := &ScriptTest{
		T:          t,
		ScriptPath: scriptPath,
		Env:        os.Environ(),
		Timeout:    10 * time.Second,
	}

	// Register cleanup
	t.Cleanup(func() {
		if test.httpRR != nil {
			test.httpRR.Close()
		}
		os.RemoveAll(tmpDir)
	})

	return test, nil
}

// WithHTTPRecording enables HTTP recording/replaying
func (s *ScriptTest) WithHTTPRecording(traceFile string) *ScriptTest {
	// Ensure the directory exists
	dir := filepath.Dir(traceFile)
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			s.T.Fatalf("Failed to create directory for HTTP trace file: %v", err)
		}
	}
	s.HTTPTraceFile = traceFile
	return s
}

// WithTimeout sets the execution timeout
func (s *ScriptTest) WithTimeout(timeout time.Duration) *ScriptTest {
	s.Timeout = timeout
	return s
}

// WithEnv adds an environment variable
func (s *ScriptTest) WithEnv(key, value string) *ScriptTest {
	s.Env = append(s.Env, key+"="+value)
	return s
}

// Result contains the results of running a script
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes the script and returns the result
func (s *ScriptTest) Run() (*Result, error) {
	// Set up HTTP recording if needed
	if s.HTTPTraceFile != "" {
		// Check if we should record or replay
		recording, err := rr.Recording(s.HTTPTraceFile)
		if err != nil {
			return nil, fmt.Errorf("failed to check recording status: %w", err)
		}

		httpRR, err := rr.Open(s.HTTPTraceFile, http.DefaultTransport)
		if err != nil {
			return nil, fmt.Errorf("failed to open HTTP record/replay: %w", err)
		}
		s.httpRR = httpRR

		// Log recording status
		if recording {
			s.T.Logf("Recording HTTP interactions to %s", s.HTTPTraceFile)
		} else {
			s.T.Logf("Replaying HTTP interactions from %s", s.HTTPTraceFile)
		}
	}

	// Create command context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	// Create the command
	// Find bash in PATH rather than hardcoding to /bin/bash
	bashPath, lookErr := exec.LookPath("bash")
	if lookErr != nil {
		return nil, fmt.Errorf("bash not available in PATH: %w", lookErr)
	}
	// Create the command
	cmd := exec.CommandContext(ctx, bashPath, s.ScriptPath)
	cmd.Env = s.Env

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil // Clear error if it's just an exit code
		}
	}

	// Create result
	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}

	return result, err
}

// ExpectOutput checks if the output contains expected strings
func (r *Result) ExpectOutput(expected ...string) bool {
	for _, exp := range expected {
		if !strings.Contains(r.Stdout, exp) {
			return false
		}
	}
	return true
}

// ExpectErrorOutput checks if the error output contains expected strings
func (r *Result) ExpectErrorOutput(expected ...string) bool {
	for _, exp := range expected {
		if !strings.Contains(r.Stderr, exp) {
			return false
		}
	}
	return true
}

// ExpectExitCode checks if the exit code matches the expected value
func (r *Result) ExpectExitCode(expected int) bool {
	return r.ExitCode == expected
}

// CreateSessionTestProgram creates a test program that uses the interactive session
func CreateSessionTestProgram(dir string, processFunc string) (string, error) {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the main.go file
	mainPath := filepath.Join(dir, "main.go")
	mainContent := fmt.Sprintf(`package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/tmc/cgpt/interactive"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	
	// Create session config
	cfg := interactive.Config{
		Prompt:      "> ",
		AltPrompt:   "... ",
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Logger:      logger,
		HistoryFile: "",
		ProcessFn: func(ctx context.Context, input string) error {
			%s
			return nil
		},
	}

	// Create and run the session
	session, err := interactive.NewSession(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %%v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := session.Run(ctx); err != nil {
		if err.Error() != "EOF" {
			fmt.Fprintf(os.Stderr, "Session failed: %%v\n", err)
			os.Exit(1)
		}
	}
}
`, processFunc)

	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write main.go: %w", err)
	}

	return mainPath, nil
}

// PipeToProgram creates a script that pipes input to a Go program
func PipeToProgram(inputFile, programPath string) (string, error) {
	dir := filepath.Dir(inputFile)
	scriptPath := filepath.Join(dir, "run_session.sh")

	scriptContent := fmt.Sprintf(`#!/bin/bash
# This script runs the session and feeds it input from our script file
cat %s | go run %s
`, inputFile, programPath)

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	return scriptPath, nil
}
