package cgpt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// historyManager handles the new history system with sessions and named symlinks
type historyManager struct {
	homeDir string
}

// newHistoryManager creates a new history manager
func newHistoryManager() (*historyManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &historyManager{homeDir: home}, nil
}

// sessionPath generates a unique session history path
func (h *historyManager) sessionPath() (string, *os.File, error) {
	sessionsDir := filepath.Join(h.homeDir, ".cgpt", "history", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	base := time.Now().Format("20060102150405")
	path := filepath.Join(sessionsDir, base+".yaml")

	// Try without suffix first
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err == nil {
		return path, f, nil
	}

	// If exists, try with numeric suffix
	for i := 1; i < 100; i++ {
		path = filepath.Join(sessionsDir, fmt.Sprintf("%s-%d.yaml", base, i))
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			return path, f, nil
		}
		if !os.IsExist(err) {
			return "", nil, err // Real error
		}
	}

	// Fallback to PID if too many collisions
	path = filepath.Join(sessionsDir, fmt.Sprintf("%s-%d.yaml", base, os.Getpid()))
	f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	return path, f, err
}

// createNamedLink creates a symlink in the named directory
func (h *historyManager) createNamedLink(sessionPath, name string) error {
	namedDir := filepath.Join(h.homeDir, ".cgpt", "history", "named")
	if err := os.MkdirAll(namedDir, 0755); err != nil {
		return fmt.Errorf("failed to create named directory: %w", err)
	}

	// Extract timestamp from session filename
	base := filepath.Base(sessionPath)
	timestamp := strings.TrimSuffix(base, ".yaml")
	if idx := strings.IndexRune(timestamp, '-'); idx > 0 {
		timestamp = timestamp[:idx] // Handle "20250827180600-1.yaml"
	}

	// Secure path sanitization to prevent directory traversal
	name = h.sanitizeFileName(name)

	// Create link with timestamp prefix
	linkName := fmt.Sprintf("%s-%s.yaml", timestamp, name)
	linkPath := filepath.Join(namedDir, linkName)

	// Create relative path for symlink
	relPath := filepath.Join("..", "sessions", base)

	// Remove existing link if it exists
	os.Remove(linkPath)

	return os.Symlink(relPath, linkPath)
}

// sanitizeFileName removes dangerous characters and path traversal attempts from a filename.
// This prevents directory traversal attacks when creating named history links.
func (h *historyManager) sanitizeFileName(name string) string {
	if name == "" {
		return "unnamed"
	}

	// Remove null bytes and other control characters
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 { // Control characters and DEL
			return -1 // Remove character
		}
		return r
	}, name)

	// Replace dangerous filesystem characters with safe alternatives
	replacements := map[string]string{
		"/":  "-",
		"\\": "-",
		":":  "-",
		"*":  "_",
		"?":  "_",
		"\"": "'",
		"<":  "(",
		">":  ")",
		"|":  "-",
	}

	for old, new := range replacements {
		name = strings.ReplaceAll(name, old, new)
	}

	// Remove any sequence containing .. to prevent directory traversal
	// This handles cases like "../", "..\\", "....//", etc.
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", ".")
	}

	// Clean up consecutive dots, dashes, and underscores
	name = strings.ReplaceAll(name, "---", "-")
	name = strings.ReplaceAll(name, "___", "_")
	name = strings.ReplaceAll(name, "...", ".")

	// Trim leading/trailing dots, dashes, and spaces
	name = strings.Trim(name, ".-_ \t")

	// Ensure the name isn't empty after cleaning
	if name == "" {
		name = "cleaned"
	}

	// Limit length to prevent filesystem issues
	if len(name) > 100 {
		name = name[:100]
		// Trim again in case we cut in the middle of a multi-byte character
		name = strings.Trim(name, ".-_ \t")
	}

	return name
}

// resolveHistoryPath determines where to save history based on the flag value
func (h *historyManager) resolveHistoryPath(historyFlag string) (string, *os.File, bool, error) {
	switch historyFlag {
	case "", "none":
		return "", nil, false, nil // No history
	case "-":
		return "-", os.Stdout, true, nil // Write to stdout
	case "auto":
		path, f, err := h.sessionPath()
		return path, f, true, err
	default:
		// User-specified path
		f, err := os.OpenFile(historyFlag, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return "", nil, false, err
		}
		return historyFlag, f, true, nil
	}
}
