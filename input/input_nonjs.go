//go:build !js

package input

import (
	"io"
	"os"
)

// stdinAvailable checks if stdin appears to have data available
// Returns true if it's a pipe or redirected file, false for terminal
func stdinAvailable(stdin io.Reader) bool {
	if stdin == nil {
		return false
	}
	// Check if it's the actual os.Stdin file
	if f, ok := stdin.(*os.File); ok && f == os.Stdin {
		stat, err := f.Stat()
		if err != nil {
			// Can't stat stdin, assume not available
			return false
		}
		// If stdin is a character device (terminal), it's not "available"
		// If it's a pipe or file, it is available
		return (stat.Mode() & os.ModeCharDevice) == 0
	}
	// If not os.Stdin but some other reader, assume it has data
	return true
}
