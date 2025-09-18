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

	// Clean the name (remove any path separators, etc)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "..", "")

	// Create link with timestamp prefix
	linkName := fmt.Sprintf("%s-%s.yaml", timestamp, name)
	linkPath := filepath.Join(namedDir, linkName)

	// Create relative path for symlink
	relPath := filepath.Join("..", "sessions", base)

	// Remove existing link if it exists
	os.Remove(linkPath)

	return os.Symlink(relPath, linkPath)
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
