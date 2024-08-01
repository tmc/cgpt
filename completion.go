package cgpt

import (
    "context"
    "fmt"
    "os"
    "strings"
    "time"
)

type CompletionService struct {
    // ... existing fields ...
    gitRepo *GitRepo
}

func (s *CompletionService) Run(ctx context.Context, runCfg RunConfig) error {
    // ... existing code ...

    if s.gitRepo != nil {
        history := HistoryEntry{
            Timestamp: time.Now(),
            Messages:  s.payload.Messages,
        }
        sha, err := s.gitRepo.SaveHistory(history)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to save history: %v\n", err)
        } else {
            fmt.Printf("History saved. SHA: %s\n", sha)
        }
    }

    // ... rest of the Run function ...
}

func (s *CompletionService) loadLatestHistory() error {
    history, err := s.gitRepo.LoadHistory("")
    if err != nil {
        return err
    }
    s.payload.Messages = history.Messages
    return nil
}

func (s *CompletionService) LoadHistoryBySHA(sha string) error {
    history, err := s.gitRepo.LoadHistory(sha)
    if err != nil {
        return err
    }
    s.payload.Messages = history.Messages
    return nil
}

func (s *CompletionService) ListCommits() ([]string, error) {
    return s.gitRepo.ListCommits()
}

func (s *CompletionService) enterCommandMode() {
    for {
        fmt.Print(":")
        var cmd string
        fmt.Scanln(&cmd)

        parts := strings.Fields(cmd)
        if len(parts) == 0 {
            continue
        }

        switch parts[0] {
        case "q", "quit":
            return
        case "history":
            s.showHistory()
        case "load":
            if len(parts) > 1 {
                s.loadHistoryEntry(parts[1])
            }
        case "list":
            s.listCommits()
        default:
            fmt.Println("Unknown command")
        }
    }
}

func (s *CompletionService) showHistory() {
    commits, err := s.ListCommits()
    if err != nil {
        fmt.Printf("Error listing commits: %v\n", err)
        return
    }
    for _, commit := range commits {
        fmt.Println(commit)
    }
}

func (s *CompletionService) loadHistoryEntry(sha string) {
    err := s.LoadHistoryBySHA(sha)
    if err != nil {
        fmt.Printf("Error loading history: %v\n", err)
    } else {
        fmt.Printf("Loaded history from SHA: %s\n", sha)
    }
}

func (s *CompletionService) listCommits() {
    commits, err := s.ListCommits()
    if err != nil {
        fmt.Printf("Error listing commits: %v\n", err)
        return
    }
    for i, commit := range commits {
        fmt.Printf("%d: %s\n", i, commit)
    }
}
