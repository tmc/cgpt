package cgpt

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "atomic-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("successful write", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "test.txt")
		testData := []byte("Hello, atomic world!")

		err := AtomicWriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("AtomicWriteFile failed: %v", err)
		}

		// Verify file exists and contains correct data
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if !bytes.Equal(readData, testData) {
			t.Errorf("File contents don't match. Expected: %s, Got: %s", testData, readData)
		}

		// Verify permissions
		info, err := os.Stat(testFile)
		if err != nil {
			t.Fatalf("Failed to stat file: %v", err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("Wrong permissions. Expected: 0644, Got: %o", info.Mode().Perm())
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "overwrite.txt")

		// Write initial data
		initialData := []byte("initial content")
		err := AtomicWriteFile(testFile, initialData, 0644)
		if err != nil {
			t.Fatalf("Initial write failed: %v", err)
		}

		// Overwrite with new data
		newData := []byte("new content that is longer than the initial content")
		err = AtomicWriteFile(testFile, newData, 0644)
		if err != nil {
			t.Fatalf("Overwrite failed: %v", err)
		}

		// Verify file contains new data
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if !bytes.Equal(readData, newData) {
			t.Errorf("File contents don't match after overwrite. Expected: %s, Got: %s", newData, readData)
		}
	})

	t.Run("create directory if needed", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "subdir", "nested", "test.txt")
		testData := []byte("nested file")

		err := AtomicWriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("AtomicWriteFile with nested dirs failed: %v", err)
		}

		// Verify file was created
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read nested file: %v", err)
		}

		if !bytes.Equal(readData, testData) {
			t.Errorf("Nested file contents don't match. Expected: %s, Got: %s", testData, readData)
		}
	})

	t.Run("handles large files", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "large.txt")

		// Create 1MB of test data
		testData := bytes.Repeat([]byte("Large file test data. "), 50000)

		err := AtomicWriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("Large file write failed: %v", err)
		}

		// Verify file size and contents
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read large file: %v", err)
		}

		if !bytes.Equal(readData, testData) {
			t.Error("Large file contents don't match")
		}
	})

	t.Run("no temp files left behind on success", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "cleanup.txt")
		testData := []byte("cleanup test")

		// Count temp files before
		tempFilesBefore := countTempFiles(tmpDir)

		err := AtomicWriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("AtomicWriteFile failed: %v", err)
		}

		// Count temp files after
		tempFilesAfter := countTempFiles(tmpDir)

		if tempFilesAfter > tempFilesBefore {
			t.Errorf("Temp files left behind. Before: %d, After: %d", tempFilesBefore, tempFilesAfter)
		}
	})

	t.Run("permission denied directory", func(t *testing.T) {
		// Create a read-only directory
		roDir := filepath.Join(tmpDir, "readonly")
		err := os.Mkdir(roDir, 0444)
		if err != nil {
			t.Fatalf("Failed to create readonly dir: %v", err)
		}
		defer os.Chmod(roDir, 0755) // Restore permissions for cleanup

		testFile := filepath.Join(roDir, "test.txt")
		testData := []byte("should fail")

		err = AtomicWriteFile(testFile, testData, 0644)
		if err == nil {
			t.Error("Expected error writing to read-only directory, but succeeded")
		}

		// Verify no file was created
		if _, err := os.Stat(testFile); err == nil {
			t.Error("File should not exist in read-only directory")
		}
	})
}

func TestAtomicWriteFileHandle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "atomic-handle-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("successful write with file handle", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "handle.txt")
		testData := []byte("Handle write test")

		// Create and open file
		f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		defer f.Close()

		err = AtomicWriteFileHandle(f, testData)
		if err != nil {
			t.Fatalf("AtomicWriteFileHandle failed: %v", err)
		}

		// Verify file contents
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if !bytes.Equal(readData, testData) {
			t.Errorf("File contents don't match. Expected: %s, Got: %s", testData, readData)
		}
	})

	t.Run("stdout writes directly", func(t *testing.T) {
		testData := []byte("stdout test")

		// This should not fail and should write directly
		err := AtomicWriteFileHandle(os.Stdout, testData)
		if err != nil {
			t.Errorf("Writing to stdout should not fail: %v", err)
		}
	})

	t.Run("handles existing file content", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "existing.txt")

		// Write initial content directly
		initialData := []byte("initial content that will be replaced")
		err := os.WriteFile(testFile, initialData, 0644)
		if err != nil {
			t.Fatalf("Failed to write initial file: %v", err)
		}

		// Open file and write atomically
		f, err := os.OpenFile(testFile, os.O_WRONLY, 0644)
		if err != nil {
			t.Fatalf("Failed to open file: %v", err)
		}
		defer f.Close()

		newData := []byte("new content")
		err = AtomicWriteFileHandle(f, newData)
		if err != nil {
			t.Fatalf("AtomicWriteFileHandle failed: %v", err)
		}

		// Verify only new content exists
		readData, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		if !bytes.Equal(readData, newData) {
			t.Errorf("File should contain only new data. Expected: %s, Got: %s", newData, readData)
		}
	})
}

// countTempFiles counts files matching the temp file pattern in a directory
func countTempFiles(dir string) int {
	count := 0
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			name := d.Name()
			if name[0] == '.' && (name[len(name)-4:] == ".tmp" || name[len(name)-4:] == ".tmp") {
				count++
			}
		}
		return nil
	})
	return count
}

// Benchmark tests
func BenchmarkAtomicWriteFile(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "atomic-bench-")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testData := []byte("benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "bench.txt")
		err := AtomicWriteFile(testFile, testData, 0644)
		if err != nil {
			b.Fatalf("AtomicWriteFile failed: %v", err)
		}
		os.Remove(testFile) // Clean up for next iteration
	}
}

func BenchmarkRegularWriteFile(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "regular-bench-")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testData := []byte("benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFile := filepath.Join(tmpDir, "bench.txt")
		err := os.WriteFile(testFile, testData, 0644)
		if err != nil {
			b.Fatalf("WriteFile failed: %v", err)
		}
		os.Remove(testFile) // Clean up for next iteration
	}
}