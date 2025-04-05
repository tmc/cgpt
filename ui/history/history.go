// Package history provides functions for loading and saving command history
// in a format compatible with libedit/readline.
package history

import (
	"bufio"
	"bytes"
	"errors" // Import errors
	"fmt"
	"io"
	"os"
	"path/filepath" // Use filepath for path manipulation
)

// Cookie used by libedit/readline format.
const cookie = "_HiStOrY_V2_"

// Load reads history entries from the specified file path.
// It expects the libedit/readline format. Returns nil slice if file not found.
func Load(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return empty history if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open history file '%s': %w", filePath, err)
	}
	defer f.Close()
	return loadFromFile(f)
}

// loadFromFile reads history entries from an io.Reader.
func loadFromFile(r io.Reader) ([]string, error) {
	// Check for cookie
	var cookieBuf [len(cookie) + 1]byte    // +1 for newline
	n, err := io.ReadFull(r, cookieBuf[:]) // Use ReadFull for exact read
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			// If the file is smaller than the cookie, it's either empty or invalid format
			if n == 0 && errors.Is(err, io.EOF) {
				return nil, nil // Empty file is valid
			}
			// File is too short or doesn't start with cookie
			fmt.Fprintf(os.Stderr, "Warning: history file format not recognized or corrupted.\n")
			return nil, nil // Treat as empty for resilience
		}
		return nil, fmt.Errorf("reading history cookie: %w", err)
	}

	if !bytes.Equal(cookieBuf[:], []byte(cookie+"\n")) {
		fmt.Fprintf(os.Stderr, "Warning: history file format not recognized (bad cookie).\n")
		return nil, nil // Treat as empty
	}

	// Read the remainder of the file line by line
	scanner := bufio.NewScanner(r)
	var history []string
	for scanner.Scan() {
		line := scanner.Bytes() // Use bytes to handle octal decoding correctly
		decodedLine := decodeOctal(line)
		history = append(history, string(decodedLine))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning history file: %w", err)
	}

	return history, nil
}

// decodeOctal converts libedit's octal escapes (e.g., \040 for space) back to bytes.
func decodeOctal(line []byte) []byte {
	var result []byte
	for i := 0; i < len(line); {
		if line[i] == '\\' && i+3 < len(line) {
			isOctal := true
			var b byte
			// Try to parse 3 octal digits
			for j := 1; j <= 3; j++ {
				digit := line[i+j]
				if digit >= '0' && digit <= '7' {
					b = (b << 3) | (digit - '0')
				} else {
					isOctal = false
					break
				}
			}
			// If it was a valid 3-digit octal escape
			if isOctal {
				result = append(result, b)
				i += 4   // Skip '\' + 3 octal digits
				continue // Continue to next part of the line
			}
		}
		// If not an octal escape, append the character as is
		result = append(result, line[i])
		i++
	}
	return result
}

// Save writes the history list to the specified file path.
// It uses the libedit/readline format.
func Save(h []string, filePath string) error {
	if filePath == "" {
		return errors.New("history save path cannot be empty")
	}
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil { // Use more restrictive permissions
		return fmt.Errorf("failed to create history directory '%s': %w", dir, err)
	}

	// Open file with truncation
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600) // Use restrictive permissions
	if err != nil {
		return fmt.Errorf("failed to open history file '%s' for writing: %w", filePath, err)
	}
	defer f.Close() // Ensure file is closed

	return saveToFile(h, f)
}

// saveToFile writes history entries to an io.Writer in libedit format.
func saveToFile(h []string, f io.Writer) error {
	w := bufio.NewWriter(f)
	_, err := w.WriteString(cookie + "\n")
	if err != nil {
		return fmt.Errorf("writing history cookie: %w", err)
	}

	for _, entry := range h {
		var buf bytes.Buffer
		for _, b := range []byte(entry) {
			// Escape characters that libedit escapes: space, tab, newline, backslash,
			// and non-printable characters.
			if b == '\\' || b == ' ' || b == '\t' || b == '\n' || b < ' ' || b > '~' {
				// Format as \NNN octal
				buf.WriteByte('\\')
				buf.WriteByte(((b >> 6) & 7) + '0') // Most significant octal digit
				buf.WriteByte(((b >> 3) & 7) + '0') // Middle octal digit
				buf.WriteByte(((b >> 0) & 7) + '0') // Least significant octal digit
			} else {
				buf.WriteByte(b) // Append printable characters directly
			}
		}
		buf.WriteByte('\n') // Add newline after each entry

		if _, err := w.Write(buf.Bytes()); err != nil {
			return fmt.Errorf("writing history entry: %w", err)
		}
	}
	return w.Flush() // Ensure all data is written to the underlying writer
}
