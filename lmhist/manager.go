package lmhist

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"gopkg.in/yaml.v3"
)

// Store represents a session storage backend.
type Store interface {
	// Save saves a session with the given ID.
	Save(ctx context.Context, id string, session *Session) error

	// Load loads a session by ID.
	Load(ctx context.Context, id string) (*Session, error)

	// List returns all session IDs.
	List(ctx context.Context) ([]string, error)

	// Delete removes a session by ID.
	Delete(ctx context.Context, id string) error
}

// Manager handles session operations using a pluggable store.
type Manager struct {
	store   Store
	current *Session
}

// NewManager creates a new history manager with the given store.
func NewManager(store Store) *Manager {
	return &Manager{
		store: store,
	}
}

// FileStore implements Store using the filesystem.
type FileStore struct {
	fsys fs.FS    // Read operations
	dir  string   // Write operations (must be concrete path)
}

// NewFileStore creates a new filesystem-based store.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		dir = path.Join(home, ".cgpt", "history")
	}

	// Ensure directory exists for writes
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}

	return &FileStore{
		fsys: os.DirFS(dir),
		dir:  dir,
	}, nil
}

// Save implements Store.Save.
func (s *FileStore) Save(ctx context.Context, id string, session *Session) error {
	if !isValidID(id) {
		return fmt.Errorf("invalid session ID: %q", id)
	}

	data, err := yaml.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	filename := path.Join(s.dir, id+".yaml")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load implements Store.Load.
func (s *FileStore) Load(ctx context.Context, id string) (*Session, error) {
	if !isValidID(id) {
		return nil, fmt.Errorf("invalid session ID: %q", id)
	}

	filename := id + ".yaml"
	data, err := fs.ReadFile(s.fsys, filename)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := yaml.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

// List implements Store.List.
func (s *FileStore) List(ctx context.Context) ([]string, error) {
	entries, err := fs.ReadDir(s.fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		id := strings.TrimSuffix(name, ".yaml")
		if isValidID(id) {
			ids = append(ids, id)
		}
	}

	return ids, nil
}

// Delete implements Store.Delete.
func (s *FileStore) Delete(ctx context.Context, id string) error {
	if !isValidID(id) {
		return fmt.Errorf("invalid session ID: %q", id)
	}

	filename := path.Join(s.dir, id+".yaml")
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("remove session file: %w", err)
	}

	return nil
}

// isValidID checks if a session ID is valid (safe for filesystem use).
func isValidID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}

	// Must be valid filename without directory separators
	return !strings.ContainsAny(id, "/\\")
}

// NewSession creates a new session with the given backend and model.
func (m *Manager) NewSession(backend, model string) *Session {
	session := &Session{
		Metadata: &Metadata{
			Created: time.Now(),
			Backend: backend,
			Model:   model,
		},
		Messages: make([]MessageInfo, 0),
	}
	m.current = session
	return session
}

// LoadSession loads a session by ID and sets it as current.
func (m *Manager) LoadSession(ctx context.Context, id string) (*Session, error) {
	session, err := m.store.Load(ctx, id)
	if err != nil {
		return nil, err
	}

	m.current = session
	return session, nil
}

// SaveSession saves the current session with the given ID.
func (m *Manager) SaveSession(ctx context.Context, id string) error {
	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	return m.store.Save(ctx, id, m.current)
}

// SaveSessionToWriter writes the current session to w in YAML format.
func (m *Manager) SaveSessionToWriter(w io.Writer) error {
	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	data, err := yaml.Marshal(m.current)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("write session: %w", err)
	}

	return nil
}

// AddMessage adds a message to the current session.
func (m *Manager) AddMessage(role llms.ChatMessageType, content string, usage *TokenUsage) error {
	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	msgInfo := MessageInfo{
		Index:     len(m.current.Messages),
		Role:      role,
		Content:   LiteralString(content),
		Timestamp: time.Now(),
		Usage:     usage,
	}

	// Store raw message for compatibility
	rawMsg := llms.TextParts(role, content)
	m.current.RawMessages = append(m.current.RawMessages, rawMsg)
	m.current.Messages = append(m.current.Messages, msgInfo)

	// Update total usage atomically
	if usage != nil {
		m.updateTotalUsage(usage)
	}

	return nil
}

// AddMessageWithPrefill adds a message with prefill metadata to the current session.
func (m *Manager) AddMessageWithPrefill(role llms.ChatMessageType, content string, usage *TokenUsage, prefill *PrefillMetadata) error {
	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	msgInfo := MessageInfo{
		Index:     len(m.current.Messages),
		Role:      role,
		Content:   LiteralString(content),
		Timestamp: time.Now(),
		Usage:     usage,
		Prefill:   prefill,
	}

	// Store raw message for compatibility
	rawMsg := llms.TextParts(role, content)
	m.current.RawMessages = append(m.current.RawMessages, rawMsg)
	m.current.Messages = append(m.current.Messages, msgInfo)

	// Update total usage atomically
	if usage != nil {
		m.updateTotalUsage(usage)
	}

	// Update prefill usage metadata
	if prefill != nil && !prefill.Skipped {
		m.current.Metadata.PrefillUsed = true
	}

	return nil
}

// updateTotalUsage updates session total usage stats.
func (m *Manager) updateTotalUsage(usage *TokenUsage) {
	if m.current.TotalUsage == nil {
		m.current.TotalUsage = &TokenUsage{}
	}

	total := m.current.TotalUsage
	total.InputTokens += usage.InputTokens
	total.OutputTokens += usage.OutputTokens
	total.TotalTokens += usage.TotalTokens
	total.ThinkingTokens += usage.ThinkingTokens
	total.CachedInputTokens += usage.CachedInputTokens

	// Update metadata
	m.current.Metadata.TotalTokens = total.TotalTokens
}

// SetThinkingConfig sets thinking mode configuration for the session.
func (m *Manager) SetThinkingConfig(mode string, budget int) error {
	if m.current == nil {
		return fmt.Errorf("no current session")
	}

	m.current.Metadata.ThinkingMode = mode
	m.current.Metadata.ThinkingBudget = budget
	return nil
}

// CurrentSession returns the current session (may be nil).
func (m *Manager) CurrentSession() *Session {
	return m.current
}

// GenerateID creates a unique session identifier.
func GenerateID(description string) string {
	timestamp := time.Now().Format("20060102-150405")
	if description == "" {
		return timestamp
	}

	// Sanitize description for filesystem safety
	clean := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, description)

	// Limit length
	if len(clean) > 30 {
		clean = clean[:30]
	}

	return timestamp + "-" + clean
}