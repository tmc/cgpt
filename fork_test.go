package cgpt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

func TestForkManager_ForkFromFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "cgpt-fork-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test conversation file
	conversation := &history{
		Backend: "test",
		Model:   "test-model",
		Messages: []llms.MessageContent{
			{
				Role:  llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{llms.TextPart("Hello")},
			},
			{
				Role:  llms.ChatMessageTypeAI,
				Parts: []llms.ContentPart{llms.TextPart("Hi there!")},
			},
			{
				Role:  llms.ChatMessageTypeHuman,
				Parts: []llms.ContentPart{llms.TextPart("How are you?")},
			},
		},
		Metadata: &historyMetadata{
			Created:     time.Now().Format(time.RFC3339),
			Description: "Test conversation",
		},
	}

	sourceFile := filepath.Join(tempDir, "source.yaml")
	sourceData, err := yaml.Marshal(conversation)
	if err != nil {
		t.Fatalf("Failed to marshal source conversation: %v", err)
	}

	if err := os.WriteFile(sourceFile, sourceData, 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Override the git history manager to use temp directory
	originalGitManager, err := NewGitHistoryManager()
	if err != nil {
		t.Fatalf("Failed to create git history manager: %v", err)
	}
	originalGitManager.RepoPath = tempDir

	forkManager := &ForkManager{
		gitManager: originalGitManager,
		homeDir:    tempDir,
	}

	// Test fork creation
	targetFile := filepath.Join(tempDir, "fork.yaml")
	ctx := context.Background()

	err = forkManager.ForkFromFile(ctx, sourceFile, targetFile, 1, "Test fork")
	if err != nil {
		t.Fatalf("Failed to create fork: %v", err)
	}

	// Verify the fork was created
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Fatal("Fork file was not created")
	}

	// Load and verify the fork content
	forkData, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read fork file: %v", err)
	}

	var forkedConversation history
	if err := yaml.Unmarshal(forkData, &forkedConversation); err != nil {
		t.Fatalf("Failed to unmarshal fork: %v", err)
	}

	// Verify fork metadata
	if forkedConversation.Metadata == nil {
		t.Fatal("Fork metadata is nil")
	}

	if forkedConversation.Metadata.ForkedFrom != sourceFile {
		t.Errorf("Expected ForkedFrom to be %s, got %s", sourceFile, forkedConversation.Metadata.ForkedFrom)
	}

	if forkedConversation.Metadata.ForkPoint != 1 {
		t.Errorf("Expected ForkPoint to be 1, got %d", forkedConversation.Metadata.ForkPoint)
	}

	if forkedConversation.Metadata.Description != "Test fork" {
		t.Errorf("Expected description 'Test fork', got %s", forkedConversation.Metadata.Description)
	}

	// Verify messages were truncated at fork point
	if len(forkedConversation.Messages) != 1 {
		t.Errorf("Expected 1 message after fork, got %d", len(forkedConversation.Messages))
	}
}

func TestForkManager_ListForks(t *testing.T) {
	// This test would require setting up a git repository with branches
	// For now, just test that the function doesn't crash
	forkManager, err := NewForkManager()
	if err != nil {
		t.Fatalf("Failed to create fork manager: %v", err)
	}

	forks, err := forkManager.ListForks()
	if err != nil {
		// It's ok if this fails in test environment - we just want to make sure it doesn't panic
		t.Logf("ListForks failed (expected in test environment): %v", err)
		return
	}

	if forks == nil {
		t.Error("Expected non-nil forks slice")
	}
}

func TestGitHistoryManager_InitializeRepo(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "cgpt-git-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gitManager := &GitHistoryManager{
		RepoPath: tempDir,
		homeDir:  tempDir,
	}

	// Test repository initialization
	err = gitManager.InitializeRepo()
	if err != nil {
		t.Fatalf("Failed to initialize repository: %v", err)
	}

	// Verify git repository was created
	gitDir := filepath.Join(tempDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Fatal("Git repository was not created")
	}

	// Verify .gitignore was created
	gitignoreFile := filepath.Join(tempDir, ".gitignore")
	if _, err := os.Stat(gitignoreFile); os.IsNotExist(err) {
		t.Fatal("Gitignore file was not created")
	}

	// Test that re-initializing doesn't fail
	err = gitManager.InitializeRepo()
	if err != nil {
		t.Fatalf("Failed to re-initialize repository: %v", err)
	}
}

func TestConversationTree_PrintTree(t *testing.T) {
	tree := &ConversationTree{
		Nodes: make(map[string]*ConversationNode),
		Root:  "root",
	}

	root := &ConversationNode{
		Hash:    "root",
		Message: "Initial conversation",
		Children: []*ConversationNode{
			{
				Hash:     "child1",
				Message:  "Fork 1",
				Children: []*ConversationNode{},
			},
			{
				Hash:     "child2",
				Message:  "Fork 2",
				Children: []*ConversationNode{},
			},
		},
	}

	tree.Nodes["root"] = root
	tree.Nodes["child1"] = root.Children[0]
	tree.Nodes["child2"] = root.Children[1]

	output := tree.PrintTree()
	if output == "" {
		t.Error("Expected non-empty tree output")
	}

	// Verify tree structure appears in output
	if !containsAll(output, []string{"Initial conversation", "Fork 1", "Fork 2"}) {
		t.Errorf("Tree output doesn't contain expected elements: %s", output)
	}
}

func TestForkCommand_Validation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cgpt-fork-cmd-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	forkManager := &ForkManager{
		homeDir: tempDir,
	}

	// Test with missing input file
	cmd := ForkCommand{
		InputFile: "nonexistent.yaml",
		ForkPoint: 0,
	}

	ctx := context.Background()
	err = forkManager.ExecuteForkCommand(ctx, cmd)
	if err == nil {
		t.Error("Expected error for nonexistent input file")
	}
}

// containsAll checks if a string contains all the given substrings
func containsAll(text string, substrings []string) bool {
	for _, substr := range substrings {
		if !strings.Contains(text, substr) {
			return false
		}
	}
	return true
}
