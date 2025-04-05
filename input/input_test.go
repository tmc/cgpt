package input

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestInputProcessor(t *testing.T) {
	ctx := context.Background()

	// Helper to read the entire contents of a reader
	readAll := func(r io.Reader) string {
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("Failed to read: %v", err)
		}
		return string(b)
	}

	t.Run("Empty", func(t *testing.T) {
		p := NewProcessor(nil, nil, nil, nil, true, false)
		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}

		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		content := readAll(r)
		if content != "" {
			t.Errorf("Expected empty output, got: %q", content)
		}
	})

	t.Run("Files", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "input-test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write content to the file
		content := "test file content"
		if _, err := tmpFile.Write([]byte(content)); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}

		// Test with the file
		p := NewProcessor([]string{tmpFile.Name()}, nil, nil, nil, true, false)
		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		if readAll(r) != content {
			t.Errorf("Expected %q, got %q", content, readAll(r))
		}
	})

	t.Run("Strings", func(t *testing.T) {
		p := NewProcessor(nil, []string{"string1", "string2"}, nil, nil, true, false)
		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		expected := "string1string2"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})

	t.Run("Args", func(t *testing.T) {
		p := NewProcessor(nil, nil, []string{"arg1", "arg2", "arg3"}, nil, true, false)
		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		expected := "arg1 arg2 arg3"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})

	t.Run("StdinViaFileDash", func(t *testing.T) {
		stdin := strings.NewReader("stdin content")
		p := NewProcessor([]string{"-"}, nil, nil, stdin, true, false)
		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		expected := "stdin content"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})

	t.Run("Combined", func(t *testing.T) {
		// Create a temporary file
		tmpFile, err := os.CreateTemp("", "input-test-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		// Write content to the file
		fileContent := "file content\n"
		if _, err := tmpFile.Write([]byte(fileContent)); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		if err := tmpFile.Close(); err != nil {
			t.Fatalf("Failed to close temp file: %v", err)
		}

		stdin := strings.NewReader("stdin content\n")

		p := NewProcessor(
			[]string{tmpFile.Name(), "-"},      // Files
			[]string{"string1\n", "string2\n"}, // Strings
			[]string{"arg1", "arg2"},           // Args
			stdin,
			true,
			false,
		)

		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		expected := "file content\nstdin content\nstring1\nstring2\narg1 arg2"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})

	t.Run("MultipleDashHandling", func(t *testing.T) {
		stdin := strings.NewReader("stdin content")
		p := NewProcessor([]string{"-", "-"}, nil, nil, stdin, true, false)
		r, warning, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		if warning != "" {
			t.Logf("Got warning: %s", warning)
		}
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		// Only the first dash should use stdin
		expected := "stdin content"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})

	t.Run("AutoIncludeStdin", func(t *testing.T) {
		stdin := strings.NewReader("piped stdin content")
		// No explicit stdin reference in files, but stdin is not a terminal
		p := NewProcessor(nil, nil, nil, stdin, false, false)
		r, _, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}
		// No longer checking for warning messages
		if tryReattach {
			t.Logf("TTY reattachment needed")
		}

		expected := "piped stdin content"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})

	t.Run("TryReattachTTY", func(t *testing.T) {
		stdin := strings.NewReader("piped stdin content")
		// We pass forceContinuous as true to simulate -c flag
		p := NewProcessor([]string{"-"}, nil, nil, stdin, false, true)
		r, _, tryReattach, err := p.GetCombinedReader(ctx)
		if err != nil {
			t.Fatalf("GetCombinedReader failed: %v", err)
		}

		if !tryReattach {
			t.Errorf("Expected tryReattachTTY to be true when forceContinuous is true and stdin is used")
		}

		// No longer checking for warning message as it was removed

		// Read the content to make sure it's still correct
		expected := "piped stdin content"
		if readAll(r) != expected {
			t.Errorf("Expected %q, got %q", expected, readAll(r))
		}
	})
}

func TestCompatibilityWrapper(t *testing.T) {
	ctx := context.Background()

	// Test the compatibility wrapper function
	stdin := strings.NewReader("stdin content")
	r, err := GetInputReader(ctx, []string{"-"}, []string{"string"}, []string{"arg"}, stdin)
	if err != nil {
		t.Fatalf("GetInputReader failed: %v", err)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	expected := "stdin contentstringarg"
	if string(b) != expected {
		t.Errorf("Expected %q, got %q", expected, string(b))
	}
}
