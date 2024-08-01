package cgpt

import (
	"fmt"
	"strconv"
	"strings"
)

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
		case "branch":
			if len(parts) > 1 {
				s.switchBranch(parts[1])
			} else {
				s.showBranches()
			}
		default:
			fmt.Println("Unknown command")
		}
	}
}

func (s *CompletionService) showBranches() {
	branches := make(map[string]bool)
	for _, commit := range s.history.Commits {
		branches[commit.Branch] = true
	}
	fmt.Println("Available branches:")
	for branch := range branches {
		if branch == s.history.CurrentBranch {
			fmt.Printf("* %s (current)\n", branch)
		} else {
			fmt.Println(branch)
		}
	}
}

func (s *CompletionService) switchBranch(branch string) {
	for i := len(s.history.Commits) - 1; i >= 0; i-- {
		if s.history.Commits[i].Branch == branch {
			s.history.CurrentBranch = branch
			s.payload.Messages = s.history.Commits[i].Messages
			fmt.Printf("Switched to branch %s\n", branch)
			return
		}
	}
	fmt.Printf("Branch %s not found\n", branch)
}

func (s *CompletionService) showHistory() {
	for i, commit := range s.getRecentCommits(10) {
		fmt.Printf("%d: %s (%s)\n", i, commit.Timestamp, commit.Branch)
	}
}

func (s *CompletionService) loadHistoryEntry(index string) {
	i, err := strconv.Atoi(index)
	if err != nil || i < 0 || i >= len(s.history.Commits) {
		fmt.Println("Invalid history index")
		return
	}

	s.payload.Messages = s.history.Commits[i].Messages
	s.history.CurrentBranch = s.history.Commits[i].Branch
	fmt.Printf("Loaded history entry %d (Branch: %s)\n", i, s.history.CurrentBranch)
}
