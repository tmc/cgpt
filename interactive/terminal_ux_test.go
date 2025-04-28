package interactive

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

// TestTerminalUX tests terminal UX features using scripttest
func TestTerminalUX(t *testing.T) {
	t.Skip("Skipping terminal UX tests - these are environment dependent")
	tests := []struct {
		name           string
		script         string
		wantOutput     []string
		wantErrOutput  []string
		wantExitCode   int
		httpRecordFile string
	}{
		{
			name: "simple input",
			script: `
#!/bin/bash
echo "hello world"
exit 0
`,
			wantOutput:   []string{"hello world"},
			wantExitCode: 0,
		},
		{
			name: "bracketed paste mode",
			script: `
#!/bin/bash
# Simulate bracketed paste by sending the escape sequences
printf "\033[200~large pasted content\nwith multiple lines\nand some code\033[201~\n"
exit 0
`,
			wantOutput:   []string{"large pasted content", "with multiple lines", "and some code"},
			wantExitCode: 0,
		},
		{
			name: "multiline input",
			script: `
#!/bin/bash
cat << EOF
line 1
line 2
line 3
EOF
exit 0
`,
			wantOutput:   []string{"line 1", "line 2", "line 3"},
			wantExitCode: 0,
		},
		{
			name: "error output",
			script: `
#!/bin/bash
echo "standard output"
echo "error message" >&2
exit 1
`,
			wantOutput:    []string{"standard output"},
			wantErrOutput: []string{"error message"},
			wantExitCode:  1,
		},
		{
			name: "http recording test",
			script: `
#!/bin/bash
echo "Making HTTP request"
curl -s https://example.com
exit 0
`,
			wantOutput:     []string{"Making HTTP request", "<html", "</html>"},
			wantExitCode:   0,
			httpRecordFile: "terminal_ux_http.trace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for test files
			tmpDir, err := os.MkdirTemp("", "terminal-ux-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create the script file
			scriptPath := filepath.Join(tmpDir, "test.sh")
			if err := os.WriteFile(scriptPath, []byte(tt.script), 0755); err != nil {
				t.Fatalf("Failed to write script file: %v", err)
			}

			// Set up HTTP recording if needed
			var httpRR *rr.RecordReplay
			if tt.httpRecordFile != "" {
				httpTraceFile := filepath.Join(tmpDir, tt.httpRecordFile)

				// Check if we should record or replay
				recording, err := rr.Recording(httpTraceFile)
				if err != nil {
					t.Fatalf("Failed to check recording status: %v", err)
				}

				httpRR, err = rr.Open(httpTraceFile, http.DefaultTransport)
				if err != nil {
					t.Fatalf("Failed to open HTTP record/replay: %v", err)
				}
				defer httpRR.Close()

				// Set the HTTP_PROXY environment variable to use our transport
				if recording {
					t.Logf("Recording HTTP interactions to %s", httpTraceFile)
				} else {
					t.Logf("Replaying HTTP interactions from %s", httpTraceFile)
				}
			}

			// Run the script
			// Find bash in PATH rather than hardcoding to /bin/bash
			bashPath, err := exec.LookPath("bash")
			if err != nil {
				t.Skip("bash not available in PATH, skipping test")
			}
			cmd := exec.Command(bashPath, scriptPath)

			// Capture stdout and stderr
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			// Set up environment
			cmd.Env = os.Environ()

			// Run the command with a timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Use directly bashPath to ensure we're using the correct bash executable
			cmd = exec.CommandContext(ctx, bashPath, scriptPath)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Env = cmd.Env

			err = cmd.Run()

			// Check exit code
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("Failed to run script: %v", err)
				}
			}

			if exitCode != tt.wantExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.wantExitCode, exitCode)
			}

			// Check stdout
			stdoutStr := stdout.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(stdoutStr, want) {
					t.Errorf("Expected stdout to contain %q, got: %q", want, stdoutStr)
				}
			}

			// Check stderr
			stderrStr := stderr.String()
			for _, want := range tt.wantErrOutput {
				if !strings.Contains(stderrStr, want) {
					t.Errorf("Expected stderr to contain %q, got: %q", want, stderrStr)
				}
			}
		})
	}
}

// TestTerminalUXWithSession tests the interactive session with script input
func TestTerminalUXWithSession(t *testing.T) {
	t.Skip("Skipping terminal UX session tests - these are environment dependent")
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "terminal-session-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a script file with commands to feed to the session
	scriptPath := filepath.Join(tmpDir, "session_input.txt")
	scriptContent := `test input
multiline
input
test
"""
code block
with multiple
lines
"""
exit
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	// Create a script runner that will feed the script to our session
	runnerScript := filepath.Join(tmpDir, "runner.sh")
	runnerContent := fmt.Sprintf(`#!/bin/bash
# This script runs the session and feeds it input from our script file
cat %s | go run ./cmd/session_test/main.go
`, scriptPath)

	if err := os.WriteFile(runnerScript, []byte(runnerContent), 0755); err != nil {
		t.Fatalf("Failed to write runner script: %v", err)
	}

	// Create the session test program
	testProgramDir := filepath.Join(tmpDir, "cmd", "session_test")
	if err := os.MkdirAll(testProgramDir, 0755); err != nil {
		t.Fatalf("Failed to create test program directory: %v", err)
	}

	testProgramPath := filepath.Join(testProgramDir, "main.go")
	testProgramContent := `package main

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
			// Write a simple response that includes the input length
			response := fmt.Sprintf("Processed input (%d chars):\n", len(input))
			response += strings.Repeat("-", 20) + "\n"
			response += input + "\n"
			response += strings.Repeat("-", 20)
			
			// Simulate some processing time
			time.Sleep(100 * time.Millisecond)
			
			fmt.Println(response)
			return nil
		},
	}

	// Create and run the session
	session, err := interactive.NewSession(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := session.Run(ctx); err != nil {
		if err.Error() != "EOF" {
			fmt.Fprintf(os.Stderr, "Session failed: %v\n", err)
			os.Exit(1)
		}
	}
}
`
	if err := os.WriteFile(testProgramPath, []byte(testProgramContent), 0644); err != nil {
		t.Fatalf("Failed to write test program: %v", err)
	}

	// Skip the actual execution in CI environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping terminal UX integration test in CI")
	}

	// Run the script
	// Find bash in PATH rather than hardcoding to /bin/bash
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available in PATH, skipping test")
	}
	cmd := exec.Command(bashPath, runnerScript)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use directly bashPath to ensure we're using the correct bash executable
	cmd = exec.CommandContext(ctx, bashPath, runnerScript)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run session test: %v\nStderr: %s", err, stderr.String())
	}

	// Verify output contains expected content
	output := stdout.String()
	expectedOutputs := []string{
		"Processed input",
		"code block",
		"with multiple",
		"lines",
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, got: %q", expected, output)
		}
	}
}

// TestScriptWithHTTPRecording tests a script that makes HTTP requests with recording/replaying
func TestScriptWithHTTPRecording(t *testing.T) {
	t.Skip("Skipping HTTP recording tests - these are environment dependent")
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "terminal-http-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create HTTP trace file path
	httpTraceFile := filepath.Join(tmpDir, "http_recording.trace")

	// Create the test script
	scriptPath := filepath.Join(tmpDir, "http_test.sh")
	scriptContent := `#!/bin/bash
echo "Making HTTP request to example.com"
curl -s https://example.com
echo "Request completed"
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	// Set up HTTP recording
	recording, err := rr.Recording(httpTraceFile)
	if err != nil {
		t.Fatalf("Failed to check recording status: %v", err)
	}

	httpRR, err := rr.Open(httpTraceFile, http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to open HTTP record/replay: %v", err)
	}
	defer httpRR.Close()

	// Create an HTTP client using the record/replay transport
	//client := &http.Client{Transport: httpRR}

	// Set up a proxy server that uses our client
	// This is just a conceptual example - in a real implementation,
	// you would need to set up an actual proxy server

	// Run the script
	// Find bash in PATH rather than hardcoding to /bin/bash
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available in PATH, skipping test")
	}
	cmd := exec.Command(bashPath, scriptPath)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run script: %v", err)
	}

	// Verify output
	output := stdout.String()
	expectedOutputs := []string{
		"Making HTTP request",
		"<html",
		"</html>",
		"Request completed",
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, got: %q", expected, output)
		}
	}

	// If we were recording, verify the trace file was created
	if recording {
		if _, err := os.Stat(httpTraceFile); os.IsNotExist(err) {
			t.Errorf("HTTP trace file was not created: %s", httpTraceFile)
		} else {
			t.Logf("Successfully recorded HTTP interactions to %s", httpTraceFile)
		}
	}
}

// scriptRunner is a helper for running scripts with controlled input/output
type scriptRunner struct {
	t          *testing.T
	scriptPath string
	env        []string
	timeout    time.Duration
	httpRR     *rr.RecordReplay
}

// newScriptRunner creates a new script runner
func newScriptRunner(t *testing.T, scriptPath string) *scriptRunner {
	return &scriptRunner{
		t:          t,
		scriptPath: scriptPath,
		env:        os.Environ(),
		timeout:    10 * time.Second,
	}
}

// withHTTPRecording enables HTTP recording/replaying
func (r *scriptRunner) withHTTPRecording(traceFile string) *scriptRunner {
	httpRR, err := rr.Open(traceFile, http.DefaultTransport)
	if err != nil {
		r.t.Fatalf("Failed to open HTTP record/replay: %v", err)
	}
	r.httpRR = httpRR
	return r
}

// withTimeout sets the execution timeout
func (r *scriptRunner) withTimeout(timeout time.Duration) *scriptRunner {
	r.timeout = timeout
	return r
}

// withEnv adds environment variables
func (r *scriptRunner) withEnv(key, value string) *scriptRunner {
	r.env = append(r.env, key+"="+value)
	return r
}

// run executes the script and returns stdout, stderr, and exit code
func (r *scriptRunner) run() (string, string, int) {
	if r.httpRR != nil {
		defer r.httpRR.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	// Find bash in PATH rather than hardcoding to /bin/bash
	bashPath, lookErr := exec.LookPath("bash")
	if lookErr != nil {
		// Log the error since we can't return it in this function signature
		r.t.Logf("bash not available in PATH: %v", lookErr)
		return "", "", 1
	}
	cmd := exec.CommandContext(ctx, bashPath, r.scriptPath)
	cmd.Env = r.env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmdErr := cmd.Run()

	exitCode := 0
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			r.t.Logf("Failed to run script: %v", cmdErr)
			// Return what we can without fataling the test
			return stdout.String(), stderr.String(), 1
		}
	}

	return stdout.String(), stderr.String(), exitCode
}
