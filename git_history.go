package cgpt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitHistoryManager handles git-based conversation history
type GitHistoryManager struct {
	RepoPath string
	homeDir  string
}

// GitMetadata represents git-specific metadata for conversations
type GitMetadata struct {
	Branch     string `yaml:"branch,omitempty"`
	CommitHash string `yaml:"commit_hash,omitempty"`
	ParentHash string `yaml:"parent_hash,omitempty"`
	TreePath   string `yaml:"tree_path,omitempty"`
}

// ForkOptions specifies how to create a conversation fork
type ForkOptions struct {
	SourceFile    string // Source conversation file
	TargetFile    string // Target conversation file
	ForkPoint     int    // Message index to fork from (-1 for current)
	Description   string // Description of the fork
	BranchName    string // Git branch name (auto-generated if empty)
}

// NewGitHistoryManager creates a new git history manager
func NewGitHistoryManager() (*GitHistoryManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	repoPath := filepath.Join(home, ".cgpt", "history")
	return &GitHistoryManager{
		RepoPath: repoPath,
		homeDir:  home,
	}, nil
}

// InitializeRepo initializes the git repository for history tracking
func (g *GitHistoryManager) InitializeRepo() error {
	if err := os.MkdirAll(g.RepoPath, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// Check if already a git repo
	if _, err := os.Stat(filepath.Join(g.RepoPath, ".git")); err == nil {
		return nil // Already initialized
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = g.RepoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Set up initial configuration
	if err := g.runGitCommand("config", "user.name", "cgpt"); err != nil {
		return fmt.Errorf("failed to set git user.name: %w", err)
	}
	if err := g.runGitCommand("config", "user.email", "cgpt@local"); err != nil {
		return fmt.Errorf("failed to set git user.email: %w", err)
	}

	// Create initial commit with .gitignore
	gitignoreContent := `# Temporary files
*.tmp
*.bak

# OS files
.DS_Store
Thumbs.db
`
	gitignorePath := filepath.Join(g.RepoPath, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	if err := g.runGitCommand("add", ".gitignore"); err != nil {
		return fmt.Errorf("failed to add .gitignore: %w", err)
	}

	if err := g.runGitCommand("commit", "-m", "Initial commit: setup cgpt history repository"); err != nil {
		return fmt.Errorf("failed to create initial commit: %w", err)
	}

	return nil
}

// CommitConversation commits a conversation to git
func (g *GitHistoryManager) CommitConversation(filePath, message string) error {
	if err := g.InitializeRepo(); err != nil {
		return err
	}

	// Get relative path within the repo
	relPath, err := filepath.Rel(g.RepoPath, filePath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Add the file
	if err := g.runGitCommand("add", relPath); err != nil {
		return fmt.Errorf("failed to add file to git: %w", err)
	}

	// Commit with message
	if err := g.runGitCommand("commit", "-m", message); err != nil {
		// Check if there are no changes to commit
		if strings.Contains(err.Error(), "nothing to commit") {
			return nil // Not an error, just no changes
		}
		return fmt.Errorf("failed to commit conversation: %w", err)
	}

	return nil
}

// ForkConversation creates a new conversation branch from an existing one
func (g *GitHistoryManager) ForkConversation(ctx context.Context, opts ForkOptions) (string, error) {
	if err := g.InitializeRepo(); err != nil {
		return "", err
	}

	// Generate branch name if not provided
	if opts.BranchName == "" {
		timestamp := time.Now().Format("20060102-150405")
		opts.BranchName = fmt.Sprintf("fork-%s", timestamp)
	}

	// Ensure we're working with absolute paths
	sourcePath := opts.SourceFile
	if !filepath.IsAbs(sourcePath) {
		sourcePath = filepath.Join(g.RepoPath, sourcePath)
	}

	// Create new branch
	if err := g.runGitCommand("checkout", "-b", opts.BranchName); err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w", opts.BranchName, err)
	}

	// Create target file path if not specified
	targetPath := opts.TargetFile
	if targetPath == "" {
		sessionsDir := filepath.Join(g.RepoPath, "sessions")
		if err := os.MkdirAll(sessionsDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create sessions directory: %w", err)
		}

		timestamp := time.Now().Format("20060102150405")
		targetPath = filepath.Join(sessionsDir, fmt.Sprintf("fork-%s.yaml", timestamp))
	}

	// Load source conversation and truncate at fork point if specified
	if opts.ForkPoint >= 0 {
		if err := g.truncateConversationAtPoint(sourcePath, targetPath, opts.ForkPoint); err != nil {
			return "", fmt.Errorf("failed to truncate conversation at fork point: %w", err)
		}
	} else {
		// Copy the entire conversation
		sourceContent, err := os.ReadFile(sourcePath)
		if err != nil {
			return "", fmt.Errorf("failed to read source conversation: %w", err)
		}
		if err := os.WriteFile(targetPath, sourceContent, 0644); err != nil {
			return "", fmt.Errorf("failed to write target conversation: %w", err)
		}
	}

	// Update fork metadata in target file
	if err := g.updateForkMetadata(targetPath, sourcePath, opts); err != nil {
		return "", fmt.Errorf("failed to update fork metadata: %w", err)
	}

	// Commit the fork
	commitMsg := fmt.Sprintf("Fork conversation from %s", filepath.Base(sourcePath))
	if opts.Description != "" {
		commitMsg = fmt.Sprintf("%s: %s", commitMsg, opts.Description)
	}

	if err := g.CommitConversation(targetPath, commitMsg); err != nil {
		return "", fmt.Errorf("failed to commit fork: %w", err)
	}

	return targetPath, nil
}

// ListBranches returns all conversation branches
func (g *GitHistoryManager) ListBranches() ([]string, error) {
	if err := g.InitializeRepo(); err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = g.RepoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	branches := strings.Fields(strings.TrimSpace(string(output)))
	return branches, nil
}

// GetConversationTree returns the conversation tree structure
func (g *GitHistoryManager) GetConversationTree() (*ConversationTree, error) {
	if err := g.InitializeRepo(); err != nil {
		return nil, err
	}

	// Get commit graph
	cmd := exec.Command("git", "log", "--graph", "--format=%H %s %ai", "--all")
	cmd.Dir = g.RepoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	tree := &ConversationTree{
		Nodes: make(map[string]*ConversationNode),
		Root:  "",
	}

	// Parse git log output and build tree structure
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Extract commit info from log line
		node := g.parseLogLine(line)
		if node != nil {
			tree.Nodes[node.Hash] = node
			if tree.Root == "" {
				tree.Root = node.Hash
			}
		}
	}

	return tree, nil
}

// SwitchToBranch switches to a specific conversation branch
func (g *GitHistoryManager) SwitchToBranch(branch string) error {
	if err := g.InitializeRepo(); err != nil {
		return err
	}

	return g.runGitCommand("checkout", branch)
}

// GetCurrentBranch returns the current git branch
func (g *GitHistoryManager) GetCurrentBranch() (string, error) {
	if err := g.InitializeRepo(); err != nil {
		return "", err
	}

	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = g.RepoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// runGitCommand executes a git command in the repository directory
func (g *GitHistoryManager) runGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s failed: %w (output: %s)", strings.Join(args, " "), err, string(output))
	}
	return nil
}

// truncateConversationAtPoint truncates a conversation at a specific message point
func (g *GitHistoryManager) truncateConversationAtPoint(sourcePath, targetPath string, forkPoint int) error {
	// This would read the YAML, truncate messages at forkPoint, and write to targetPath
	// Implementation would depend on the conversation format
	// For now, just copy the file - this should be enhanced based on the actual message structure
	sourceContent, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, sourceContent, 0644)
}

// updateForkMetadata updates the fork metadata in a conversation file
func (g *GitHistoryManager) updateForkMetadata(targetPath, sourcePath string, opts ForkOptions) error {
	// This would update the YAML metadata to include fork information
	// For now, this is a placeholder - should integrate with existing historyMetadata structure
	return nil
}

// parseLogLine parses a git log line into a ConversationNode
func (g *GitHistoryManager) parseLogLine(line string) *ConversationNode {
	// Parse the git log --graph output
	// This is a simplified parser - could be enhanced
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 2 {
		return nil
	}

	return &ConversationNode{
		Hash:    parts[0],
		Message: strings.Join(parts[1:], " "),
		Children: make([]*ConversationNode, 0),
	}
}

// ConversationTree represents the tree structure of conversations
type ConversationTree struct {
	Nodes map[string]*ConversationNode
	Root  string
}

// ConversationNode represents a single node in the conversation tree
type ConversationNode struct {
	Hash     string
	Message  string
	Branch   string
	Created  time.Time
	Children []*ConversationNode
}

// PrintTree prints a visual representation of the conversation tree
func (t *ConversationTree) PrintTree() string {
	if t.Root == "" {
		return "No conversations found"
	}

	var result strings.Builder
	t.printNode(&result, t.Nodes[t.Root], "", true)
	return result.String()
}

func (t *ConversationTree) printNode(sb *strings.Builder, node *ConversationNode, prefix string, isLast bool) {
	if node == nil {
		return
	}

	// Determine the tree symbols
	var symbol, nextPrefix string
	if isLast {
		symbol = "└── "
		nextPrefix = prefix + "    "
	} else {
		symbol = "├── "
		nextPrefix = prefix + "│   "
	}

	// Print the node
	sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, symbol, node.Message))

	// Print children
	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		t.printNode(sb, child, nextPrefix, isLastChild)
	}
}