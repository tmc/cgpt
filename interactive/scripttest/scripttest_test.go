package scripttest

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScriptTest(t *testing.T) {
	// Skip in CI environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping script test in CI environment")
	}

	// Create a simple test script
	script := `#!/bin/bash
echo "Hello, world!"
echo "Error message" >&2
exit 0
`

	// Create a new script test
	st, err := New(t, script)
	if err != nil {
		t.Fatalf("Failed to create script test: %v", err)
	}

	// Run the script
	result, err := st.Run()
	if err != nil {
		t.Fatalf("Failed to run script: %v", err)
	}

	// Check the results
	if !result.ExpectOutput("Hello, world!") {
		t.Errorf("Expected output to contain 'Hello, world!', got: %q", result.Stdout)
	}

	if !result.ExpectErrorOutput("Error message") {
		t.Errorf("Expected error output to contain 'Error message', got: %q", result.Stderr)
	}

	if !result.ExpectExitCode(0) {
		t.Errorf("Expected exit code 0, got: %d", result.ExitCode)
	}
}

func TestHTTPRecording(t *testing.T) {
	// Skip in CI environments or if not explicitly enabled
	if os.Getenv("CI") != "" || os.Getenv("ENABLE_HTTP_TESTS") == "" {
		t.Skip("Skipping HTTP recording test - set ENABLE_HTTP_TESTS=1 to run this test")
	}

	// Get the testdata directory
	testdataDir := filepath.Join("testdata")

	// Create a temp dir for the test
	tmpDir := t.TempDir()
	
	// Ensure the temp dir exists
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create the HTTP trace file path directly in the tempdir without subdirectories
	httpTraceFile := filepath.Join(tmpDir, "http_recording.trace")

	// Create a script test using the HTTP test script
	scriptPath := filepath.Join(testdataDir, "http_test.sh")
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read test script: %v", err)
	}

	st, err := New(t, string(scriptContent))
	if err != nil {
		t.Fatalf("Failed to create script test: %v", err)
	}

	// Enable HTTP recording
	st.WithHTTPRecording(httpTraceFile)

	// Run the script
	result, err := st.Run()
	if err != nil {
		t.Fatalf("Failed to run script: %v", err)
	}

	// Check the results
	expectedOutputs := []string{
		"Making HTTP request",
		"Request completed",
		"httpbin.org",
	}

	for _, expected := range expectedOutputs {
		if !result.ExpectOutput(expected) {
			t.Errorf("Expected output to contain %q, got: %q", expected, result.Stdout)
		}
	}

	// Verify the trace file was created
	if _, err := os.Stat(httpTraceFile); os.IsNotExist(err) {
		t.Errorf("HTTP trace file was not created: %s", httpTraceFile)
	}

	// Run the script again to test replay
	t.Log("Running script again to test HTTP replay")
	result2, err := st.Run()
	if err != nil {
		t.Fatalf("Failed to run script (replay): %v", err)
	}

	// Check that replay worked
	for _, expected := range expectedOutputs {
		if !result2.ExpectOutput(expected) {
			t.Errorf("Expected replay output to contain %q, got: %q", expected, result2.Stdout)
		}
	}
}

func TestInteractiveSession(t *testing.T) {
	// Skip in CI environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping interactive session test in CI environment")
	}

	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create the input file
	inputPath := filepath.Join(tmpDir, "input.txt")
	inputContent := `test input
multiline
input
"""
code block
with multiple
lines
"""
exit
`
	if err := os.WriteFile(inputPath, []byte(inputContent), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Ensure the testdata directory exists
	testdataDir := filepath.Join("testdata")
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(testdataDir, 0755); err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}
	}

	// Create the session test program
	processFunc := `
		// Ensure we always print the "Processed input" string for test validation
		fmt.Printf("Processed input (%d chars):\\n", len(input))
		fmt.Printf("%s\\n", strings.Repeat("-", 20))
		fmt.Printf("%s\\n", input)
		fmt.Printf("%s\\n", strings.Repeat("-", 20))
		
		// Simulate some processing time
		time.Sleep(100 * time.Millisecond)
	`

	programPath, err := CreateSessionTestProgram(filepath.Join(tmpDir, "cmd", "session_test"), processFunc)
	if err != nil {
		t.Fatalf("Failed to create session test program: %v", err)
	}

	// Create the script to run the program
	scriptPath, err := PipeToProgram(inputPath, programPath)
	if err != nil {
		t.Fatalf("Failed to create runner script: %v", err)
	}

	// Create a script test
	st, err := New(t, "#!/bin/bash\n"+scriptPath)
	if err != nil {
		t.Fatalf("Failed to create script test: %v", err)
	}

	// Set a longer timeout for the interactive session
	st.WithTimeout(30 * time.Second)

	// Run the script
	result, err := st.Run()
	if err != nil {
		t.Fatalf("Failed to run interactive session: %v", err)
	}

	// Check the results
	expectedOutputs := []string{
		"Processed input",
		"code block",
		"with multiple",
		"lines",
	}

	for _, expected := range expectedOutputs {
		if !result.ExpectOutput(expected) {
			t.Errorf("Expected output to contain %q, got: %q", expected, result.Stdout)
		}
	}
}
