package interactive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tmc/cgpt/internal/rr"
)

func TestTerminalUXScriptIntegration(t *testing.T) {
	t.Skip("Skipping terminal UX integration tests - these are environment dependent")
	// Skip if not on a supported platform or in CI
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" || os.Getenv("CI") != "" {
		t.Skip("Skipping script integration test - requires interactive terminal on Linux/macOS")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "cgpt-termtest-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Paths for test files
	httprrFile := filepath.Join(tempDir, "terminal_test.httprr")
	scriptFile := filepath.Join(tempDir, "terminal_test.sh")
	inputFile := filepath.Join(tempDir, "input.txt")
	outputFile := filepath.Join(tempDir, "output.txt")

	// Write test input
	testInput := "Please provide a brief explanation of how HTTP record/replay works.\n"
	err = os.WriteFile(inputFile, []byte(testInput), 0644)
	if err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Build the cgpt binary if not already built
	cgptPath := filepath.Join("..", "cgpt")
	if _, err := os.Stat(cgptPath); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", cgptPath, "../cmd/cgpt")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to build cgpt binary: %v\n%s", err, output)
		}
	}
	cgptPath, err = filepath.Abs(cgptPath)
	if err != nil {
		t.Fatalf("Failed to resolve cgpt path: %v", err)
	}

	// Create test script
	scriptContent := fmt.Sprintf(`#!/bin/bash
# Terminal UX test script
set -e

# Use test directory
cd %s || exit 1

# Record HTTP requests if needed
if [ "$1" = "--record" ]; then
  echo "Recording HTTP interactions..."
  %s --backend=dummy --model=dummy-model --slow-responses -f input.txt --http-record="%s" > %s
  exit $?
fi

# Replay mode by default
echo "Replaying HTTP interactions..."
%s --backend=dummy --model=dummy-model -f input.txt --http-record="%s" > %s
exit $?
`, tempDir, cgptPath, httprrFile, outputFile, cgptPath, httprrFile, outputFile)

	err = os.WriteFile(scriptFile, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write script file: %v", err)
	}

	// First run in record mode
	cmd := exec.Command(scriptFile, "--record")
	var recordOutput bytes.Buffer
	cmd.Stdout = &recordOutput
	cmd.Stderr = &recordOutput

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Record mode failed: %v\nOutput: %s", err, recordOutput.String())
	}

	// Verify the HTTP record file was created
	_, err = os.Stat(httprrFile)
	if err != nil {
		t.Fatalf("HTTP record file wasn't created: %v", err)
	}

	// Verify the output file exists with content
	recordedOutput, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if len(recordedOutput) == 0 {
		t.Fatalf("Output file is empty")
	}

	// Save the recorded output for comparison
	recordedOutputStr := string(recordedOutput)

	// Clear the output file
	err = os.WriteFile(outputFile, nil, 0644)
	if err != nil {
		t.Fatalf("Failed to clear output file: %v", err)
	}

	// Now run in replay mode
	cmd = exec.Command(scriptFile)
	var replayOutput bytes.Buffer
	cmd.Stdout = &replayOutput
	cmd.Stderr = &replayOutput

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Replay mode failed: %v\nOutput: %s", err, replayOutput.String())
	}

	// Read the replay output
	replayedOutput, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read replayed output: %v", err)
	}
	if len(replayedOutput) == 0 {
		t.Fatalf("Replayed output file is empty")
	}

	// Compare the outputs
	if recordedOutputStr != string(replayedOutput) {
		t.Errorf("Recorded and replayed outputs should match\nRecorded: %s\nReplayed: %s",
			recordedOutputStr, string(replayedOutput))
	}
}

// TestHTTPRecordReplayWithSlowResponses tests the integration of HTTP record/replay
// specifically with slow responses for terminal UX testing
func TestHTTPRecordReplayWithSlowResponses(t *testing.T) {
	t.Skip("Skipping HTTP record/replay tests - these are environment dependent")
	// Create a temporary file for the record/replay
	httprrFile, err := os.CreateTemp("", "terminal-test-*.httprr")
	if err != nil {
		t.Fatalf("Failed to create HTTP record/replay file: %v", err)
	}
	defer os.Remove(httprrFile.Name())
	httprrFile.Close()

	// Setup a simple HTTP server for testing
	server := http.Server{
		Addr: "localhost:8099",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if we should simulate a slow response
			if r.URL.Query().Get("slow") == "true" {
				// Send response in chunks with delays
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)

				for i := 0; i < 5; i++ {
					fmt.Fprintf(w, "Chunk %d of response\n", i+1)
					w.(http.Flusher).Flush()
					time.Sleep(200 * time.Millisecond)
				}
			} else {
				// Fast response
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, "Fast response")
			}
		}),
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("HTTP server error: %v", err)
		}
	}()
	defer server.Shutdown(context.Background())

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// First, record a slow response
	recordReplay, err := rr.Open(httprrFile.Name(), http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create record/replay: %v", err)
	}
	isRecording := recordReplay.Recording()
	t.Logf("Recording mode: %v", isRecording)

	// Set as default HTTP client transport
	originalTransport := http.DefaultTransport
	http.DefaultTransport = recordReplay
	defer func() {
		http.DefaultTransport = originalTransport
	}()

	// Make a slow request
	resp, err := http.Get("http://localhost:8099/?slow=true")
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Record response content and time
	recordedContent := string(body)
	recordStartTime := time.Now()

	// Verify slow response
	if isRecording {
		if !strings.Contains(recordedContent, "Chunk") {
			t.Errorf("Response should contain chunks, got: %s", recordedContent)
		}

		elapsed := time.Since(recordStartTime)
		if elapsed <= 500*time.Millisecond {
			t.Errorf("Recording a slow response should take significant time, took: %v", elapsed)
		}
	}

	// Close the recorder to save the file
	recordReplay.Close()

	// Now create a new replay instance
	replayStartTime := time.Now()
	replayInstance, err := rr.Open(httprrFile.Name(), http.DefaultTransport)
	if err != nil {
		t.Fatalf("Failed to create replay instance: %v", err)
	}
	http.DefaultTransport = replayInstance
	defer replayInstance.Close()

	// Make the same request - should be replayed
	resp, err = http.Get("http://localhost:8099/?slow=true")
	if err != nil {
		t.Fatalf("Replay HTTP request failed: %v", err)
	}
	replayBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read replay response: %v", err)
	}

	replayDuration := time.Since(replayStartTime)
	replayContent := string(replayBody)

	// Verify replay content matches but is faster
	if recordedContent != replayContent {
		t.Errorf("Recorded and replayed content should match\nRecorded: %s\nReplayed: %s",
			recordedContent, replayContent)
	}

	// In replay mode, the response should be much faster
	if !replayInstance.Recording() {
		t.Logf("Replay duration: %v", replayDuration)
		if !strings.Contains(replayContent, "Chunk") {
			t.Errorf("Replayed response should contain chunks, got: %s", replayContent)
		}
	}
}
