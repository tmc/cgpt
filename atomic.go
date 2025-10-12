package cgpt

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to a file atomically by writing to a temporary file
// and then renaming it to the target file. This prevents corruption from
// interrupted writes, concurrent access, or system crashes.
//
// The function:
// 1. Creates a temporary file in the same directory as the target
// 2. Writes data to the temporary file
// 3. Syncs the temporary file to disk
// 4. Atomically renames the temporary file to the target
// 5. Cleans up the temporary file on any error
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	// Ensure the parent directory exists
	dir := filepath.Dir(filename)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", dir, err)
		}
	}

	// Create temporary file in the same directory
	tempFile, err := os.CreateTemp(dir, ".cgpt-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempName := tempFile.Name()

	// Cleanup temporary file on error
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempName)
		}
	}()

	// Write data to temporary file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Set permissions before closing
	if err := tempFile.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions on temp file: %w", err)
	}

	// Close the temporary file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	tempFile = nil // Mark as closed for defer cleanup

	// Atomically rename temporary file to target
	if err := os.Rename(tempName, filename); err != nil {
		return fmt.Errorf("failed to rename temp file to %q: %w", filename, err)
	}

	return nil
}

// AtomicWriteFileHandle writes data atomically using an existing file handle.
// This is useful when you need to write to a file that's already opened
// and want to maintain atomic write semantics.
//
// The function assumes the file handle is already positioned where you want
// to write and will truncate the file before writing.
func AtomicWriteFileHandle(f *os.File, data []byte) error {
	if f == os.Stdout || f == os.Stderr || f == os.Stdin {
		// For standard streams, just write directly
		_, err := f.Write(data)
		return err
	}

	fileName := f.Name()

	// Create temp file in same directory for atomic write
	tempFile, err := os.CreateTemp(filepath.Dir(fileName), ".cgpt-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFileName := tempFile.Name()

	// Clean up temp file on error
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempFileName)
		}
	}()

	// Write to temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	tempFile = nil // Mark as closed for defer cleanup

	// Atomically rename temp file to target
	if err := os.Rename(tempFileName, fileName); err != nil {
		return fmt.Errorf("failed to rename history file: %w", err)
	}

	return nil
}
