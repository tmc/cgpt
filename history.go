package cgpt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

type Commit struct {
	Timestamp time.Time
	Messages  []llms.MessageContent
	Branch    string
}
type RepoHistory struct {
	Commits       []Commit
	CurrentBranch string
}

func getHistoryFilePath(userSuppliedPath string) (string, error) {
	if userSuppliedPath != "" {
		// Use the user-supplied path if provided
		return userSuppliedPath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	cgptDir := filepath.Join(homeDir, ".cgpt", "history")
	if err := os.MkdirAll(cgptDir, 0755); err != nil {
		return "", err
	}
	repoPath, err := getRepoRelativePath()
	if err != nil {
		// If not in a Git repo, use a default name
		repoPath = "default"
	}
	repoPath = strings.ReplaceAll(repoPath, string(filepath.Separator), "-")
	historyFile := filepath.Join(cgptDir, repoPath+".yaml")
	return historyFile, nil
}
func getRepoRelativePath() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		// Git command failed, return a default path
		return "default", nil
	}
	repoRoot := strings.TrimSpace(string(output))
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	relPath, err := filepath.Rel(homeDir, repoRoot)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relPath), nil
}
func (s *CompletionService) saveHistory() error {
	historyFile, err := getHistoryFilePath(s.userSuppliedHistoryFile)
	if err != nil {
		return err
	}
	commit := Commit{
		Timestamp: time.Now(),
		Messages:  s.payload.Messages,
		Branch:    s.history.CurrentBranch,
	}
	s.history.Commits = append(s.history.Commits, commit)
	f, err := os.Create(historyFile)
	if err != nil {
		return err
	}
	defer f.Close()
	y, err := yaml.Marshal(s.history)
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}
	if _, err := f.Write(y); err != nil {
		return fmt.Errorf("failed to write history: %w", err)
	}
	return nil
}

func (s *CompletionService) loadHistory() error {
	historyFile, err := getHistoryFilePath(s.userSuppliedHistoryFile)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, initialize a new history with a new branch
			s.history = RepoHistory{
				CurrentBranch: fmt.Sprintf("gpt-%d", time.Now().Unix()),
			}
			return nil
		}
		return err
	}
	if err := yaml.Unmarshal(data, &s.history); err != nil {
		return err
	}
	// Start a new branch for this session
	s.history.CurrentBranch = fmt.Sprintf("gpt-%d", time.Now().Unix())
	// If there are previous commits, load the last one's messages
	if len(s.history.Commits) > 0 {
		s.payload.Messages = s.history.Commits[len(s.history.Commits)-1].Messages
	}
	return nil
}
func (s *CompletionService) getRecentCommits(n int) []Commit {
	if n > len(s.history.Commits) {
		n = len(s.history.Commits)
	}
	return s.history.Commits[len(s.history.Commits)-n:]
}
