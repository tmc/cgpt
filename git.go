package cgpt

import (
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

type GitRepo struct {
    Path string
    mu   sync.Mutex
}

type HistoryEntry struct {
    Timestamp time.Time              `json:"timestamp"`
    Messages  []llms.MessageContent  `json:"messages"`
}

func (s *CompletionService) InitGitRepo(path string) error {
    repo, err := NewGitRepo(path)
    if err != nil {
        return err
    }
    s.gitRepo = repo
    return s.loadLatestHistory()
}

func NewGitRepo(path string) (*GitRepo, error) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return nil, fmt.Errorf("repository path does not exist: %s", path)
    }
    return &GitRepo{Path: path}, nil
}

func (r *GitRepo) RunGitCommand(args ...string) (string, error) {
    r.mu.Lock()
    defer r.mu.Unlock()

    cmd := exec.Command("git", args...)
    cmd.Dir = r.Path
    output, err := cmd.CombinedOutput()
    return string(output), err
}

func (r *GitRepo) SaveHistory(history HistoryEntry) (string, error) {
    data, err := json.MarshalIndent(history, "", "  ")
    if err != nil {
        return "", err
    }

    filename := fmt.Sprintf("%d.json", history.Timestamp.Unix())
    fullPath := filepath.Join(r.Path, filename)

    err = os.WriteFile(fullPath, data, 0644)
    if err != nil {
        return "", err
    }

    _, err = r.RunGitCommand("add", filename)
    if err != nil {
        return "", err
    }

    _, err = r.RunGitCommand("commit", "-m", fmt.Sprintf("Update history: %s", history.Timestamp))
    if err != nil {
        return "", err
    }

    sha, err := r.RunGitCommand("rev-parse", "HEAD")
    if err != nil {
        return "", err
    }

    return strings.TrimSpace(sha), nil
}

func (r *GitRepo) LoadHistory(sha string) (HistoryEntry, error) {
    var history HistoryEntry

    if sha == "" {
        // Load the latest history file
        files, err := filepath.Glob(filepath.Join(r.Path, "*.json"))
        if err != nil {
            return history, err
        }
        if len(files) == 0 {
            return history, nil // Return empty history if no files found
        }
        sha = "HEAD"
    }

    // Get the file content at the specified SHA
    output, err := r.RunGitCommand("show", fmt.Sprintf("%s:*.json", sha))
    if err != nil {
        return history, err
    }

    err = json.Unmarshal([]byte(output), &history)
    return history, err
}

func (r *GitRepo) ListCommits() ([]string, error) {
    output, err := r.RunGitCommand("log", "--format=%H %s")
    if err != nil {
        return nil, err
    }
    return strings.Split(strings.TrimSpace(output), "\n"), nil
}
