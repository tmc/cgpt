package cgpt

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

// ForkManager handles conversation forking operations
type ForkManager struct {
	gitManager *GitHistoryManager
	homeDir    string
}

// NewForkManager creates a new fork manager
func NewForkManager() (*ForkManager, error) {
	gitManager, err := NewGitHistoryManager()
	if err != nil {
		return nil, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	return &ForkManager{
		gitManager: gitManager,
		homeDir:    home,
	}, nil
}

// ForkFromFile creates a fork of a conversation from a file
func (f *ForkManager) ForkFromFile(ctx context.Context, inputFile, outputFile string, forkPoint int, description string) error {
	// Ensure git repository is initialized
	if err := f.gitManager.InitializeRepo(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Load the source conversation
	conversation, err := f.loadConversation(inputFile)
	if err != nil {
		return fmt.Errorf("failed to load source conversation: %w", err)
	}

	// Create fork at specified point
	forkedConversation := f.createFork(conversation, inputFile, forkPoint, description)

	// Write the forked conversation
	if err := f.saveConversation(forkedConversation, outputFile); err != nil {
		return fmt.Errorf("failed to save forked conversation: %w", err)
	}

	// Commit to git
	commitMsg := fmt.Sprintf("Fork from %s at message %d", filepath.Base(inputFile), forkPoint)
	if description != "" {
		commitMsg = fmt.Sprintf("%s: %s", commitMsg, description)
	}

	if err := f.gitManager.CommitConversation(outputFile, commitMsg); err != nil {
		return fmt.Errorf("failed to commit fork: %w", err)
	}

	return nil
}

// ForkFromService creates a fork directly from a completion service
func (f *ForkManager) ForkFromService(ctx context.Context, service *CompletionService, outputFile string, forkPoint int, description string) error {
	if service == nil {
		return fmt.Errorf("completion service cannot be nil")
	}

	// Get current conversation state
	conversation := &history{
		Backend:  service.cfg.Backend,
		Model:    service.payload.Model,
		Messages: make([]llms.MessageContent, len(service.payload.Messages)),
	}
	copy(conversation.Messages, service.payload.Messages)

	// Truncate at fork point if specified
	if forkPoint >= 0 && forkPoint < len(conversation.Messages) {
		conversation.Messages = conversation.Messages[:forkPoint]
	}

	// Create fork metadata
	sourceFile := ""
	if service.historyFile != nil && service.historyFile != os.Stdout {
		sourceFile = service.historyFile.Name()
	}

	conversation.Metadata = &historyMetadata{
		Created:     time.Now().Format(time.RFC3339),
		Description: description,
		ForkedFrom:  sourceFile,
		ForkPoint:   forkPoint,
		UsageInfo:   &usageInfo{},
	}

	// Copy existing metadata if available
	if service.historyMetadata != nil {
		if service.historyMetadata.Tags != nil {
			conversation.Metadata.Tags = make([]string, len(service.historyMetadata.Tags))
			copy(conversation.Metadata.Tags, service.historyMetadata.Tags)
		}
		if service.historyMetadata.Topics != nil {
			conversation.Metadata.Topics = make([]string, len(service.historyMetadata.Topics))
			copy(conversation.Metadata.Topics, service.historyMetadata.Topics)
		}
	}

	// Save the forked conversation
	if err := f.saveConversation(conversation, outputFile); err != nil {
		return fmt.Errorf("failed to save forked conversation: %w", err)
	}

	// Commit to git
	commitMsg := fmt.Sprintf("Fork from active session at message %d", forkPoint)
	if description != "" {
		commitMsg = fmt.Sprintf("%s: %s", commitMsg, description)
	}

	if err := f.gitManager.CommitConversation(outputFile, commitMsg); err != nil {
		return fmt.Errorf("failed to commit fork: %w", err)
	}

	return nil
}

// ListForks returns all forks in the git history
func (f *ForkManager) ListForks() ([]ForkInfo, error) {
	branches, err := f.gitManager.ListBranches()
	if err != nil {
		return nil, err
	}

	var forks []ForkInfo
	for _, branch := range branches {
		if strings.HasPrefix(branch, "fork-") || branch == "main" {
			forkInfo := ForkInfo{
				Branch:      branch,
				Description: f.getBranchDescription(branch),
			}
			forks = append(forks, forkInfo)
		}
	}

	return forks, nil
}

// GetConversationTree returns the tree structure of all conversations
func (f *ForkManager) GetConversationTree() (*ConversationTree, error) {
	return f.gitManager.GetConversationTree()
}

// SwitchToFork switches to a specific fork branch
func (f *ForkManager) SwitchToFork(branch string) error {
	return f.gitManager.SwitchToBranch(branch)
}

// CreateForkBranch creates a new git branch for forking
func (f *ForkManager) CreateForkBranch(name string) error {
	return f.gitManager.runGitCommand("checkout", "-b", name)
}

// loadConversation loads a conversation from a file
func (f *ForkManager) loadConversation(filePath string) (*history, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var conversation history
	if err := yaml.Unmarshal(data, &conversation); err != nil {
		return nil, err
	}

	return &conversation, nil
}

// saveConversation saves a conversation to a file
func (f *ForkManager) saveConversation(conversation *history, filePath string) error {
	data, err := yaml.Marshal(conversation)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return AtomicWriteFile(filePath, data, 0644)
}

// createFork creates a forked version of a conversation
func (f *ForkManager) createFork(source *history, sourceFile string, forkPoint int, description string) *history {
	fork := &history{
		Backend:  source.Backend,
		Model:    source.Model,
		Messages: make([]llms.MessageContent, len(source.Messages)),
		Metadata: &historyMetadata{
			Created:     time.Now().Format(time.RFC3339),
			Description: description,
			ForkedFrom:  sourceFile,
			ForkPoint:   forkPoint,
			UsageInfo:   &usageInfo{},
		},
	}

	// Copy messages up to fork point
	if forkPoint >= 0 && forkPoint < len(source.Messages) {
		fork.Messages = make([]llms.MessageContent, forkPoint)
		copy(fork.Messages[:forkPoint], source.Messages[:forkPoint])
	} else {
		// Copy all messages if no specific fork point
		copy(fork.Messages, source.Messages)
	}

	// Copy metadata if available
	if source.Metadata != nil {
		if source.Metadata.Tags != nil {
			fork.Metadata.Tags = make([]string, len(source.Metadata.Tags))
			copy(fork.Metadata.Tags, source.Metadata.Tags)
		}
		if source.Metadata.Topics != nil {
			fork.Metadata.Topics = make([]string, len(source.Metadata.Topics))
			copy(fork.Metadata.Topics, source.Metadata.Topics)
		}
	}

	return fork
}

// getBranchDescription gets a description for a git branch
func (f *ForkManager) getBranchDescription(branch string) string {
	// This could be enhanced to read commit messages or metadata
	return fmt.Sprintf("Conversation branch: %s", branch)
}

// ForkInfo represents information about a conversation fork
type ForkInfo struct {
	Branch      string    `json:"branch"`
	Description string    `json:"description"`
	Created     time.Time `json:"created,omitempty"`
	ParentHash  string    `json:"parent_hash,omitempty"`
}

// ForkCommand represents a command to create a fork
type ForkCommand struct {
	InputFile   string
	OutputFile  string
	ForkPoint   int
	Description string
	BranchName  string
}

// ExecuteForkCommand executes a fork command
func (f *ForkManager) ExecuteForkCommand(ctx context.Context, cmd ForkCommand) error {
	// Create output file if not specified
	if cmd.OutputFile == "" {
		timestamp := time.Now().Format("20060102150405")
		sessionsDir := filepath.Join(f.homeDir, ".cgpt", "history", "sessions")
		cmd.OutputFile = filepath.Join(sessionsDir, fmt.Sprintf("fork-%s.yaml", timestamp))
	}

	// Create branch if specified
	if cmd.BranchName != "" {
		if err := f.CreateForkBranch(cmd.BranchName); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	}

	// Execute the fork
	return f.ForkFromFile(ctx, cmd.InputFile, cmd.OutputFile, cmd.ForkPoint, cmd.Description)
}
