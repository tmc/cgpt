package cgpt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"sigs.k8s.io/yaml"
)

// HistorySearchResult represents a search result for history sessions
type HistorySearchResult struct {
	Path        string
	Created     time.Time
	Description string
	Tags        []string
	Topics      []string
	Score       int // Relevance score for sorting
}

// SearchHistorySessions searches for history sessions matching the given query
func SearchHistorySessions(query string) ([]HistorySearchResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Check both sessions and named directories
	sessionDir := filepath.Join(home, ".cgpt", "history", "sessions")
	namedDir := filepath.Join(home, ".cgpt", "history", "named")

	var results []HistorySearchResult
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// Search in sessions directory
	if err := searchInDirectory(sessionDir, queryWords, &results); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Search in named directory (follow symlinks)
	if err := searchNamedLinks(namedDir, queryWords, &results); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Sort by relevance score (higher is better)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		// If scores are equal, sort by date (newer first)
		return results[i].Created.After(results[j].Created)
	})

	return results, nil
}

func searchInDirectory(dir string, queryWords []string, results *[]HistorySearchResult) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		// Try to load and search the history file
		if result, ok := searchHistoryFile(path, queryWords); ok {
			*results = append(*results, result)
		}

		return nil
	})
}

func searchNamedLinks(dir string, queryWords []string, results *[]HistorySearchResult) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())

		// Resolve symlink
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue // Skip broken links
		}

		// Check if we've already processed this file
		alreadyProcessed := false
		for _, r := range *results {
			if r.Path == target {
				alreadyProcessed = true
				break
			}
		}
		if alreadyProcessed {
			continue
		}

		// Try to load and search the history file
		if result, ok := searchHistoryFile(target, queryWords); ok {
			// Bonus points for named files
			result.Score += 5
			*results = append(*results, result)
		}
	}

	return nil
}

func searchHistoryFile(path string, queryWords []string) (HistorySearchResult, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HistorySearchResult{}, false
	}

	var h history
	if err := yaml.Unmarshal(data, &h); err != nil {
		return HistorySearchResult{}, false
	}

	result := HistorySearchResult{
		Path: path,
	}

	// Extract metadata
	if h.Metadata != nil {
		if h.Metadata.Created != "" {
			if t, err := time.Parse(time.RFC3339, h.Metadata.Created); err == nil {
				result.Created = t
			}
		}
		result.Description = h.Metadata.Description
		result.Tags = h.Metadata.Tags
		result.Topics = h.Metadata.Topics
	}

	// Calculate relevance score
	score := 0

	// Check description
	descLower := strings.ToLower(result.Description)
	for _, word := range queryWords {
		if strings.Contains(descLower, word) {
			score += 10
		}
	}

	// Check tags
	for _, tag := range result.Tags {
		tagLower := strings.ToLower(tag)
		for _, word := range queryWords {
			if strings.Contains(tagLower, word) {
				score += 15 // Tags are highly relevant
			}
		}
	}

	// Check topics
	for _, topic := range result.Topics {
		topicLower := strings.ToLower(topic)
		for _, word := range queryWords {
			if strings.Contains(topicLower, word) {
				score += 12 // Topics are also highly relevant
			}
		}
	}

	// Check message content (first few messages)
	messageCount := 0
	for _, msg := range h.Messages {
		if messageCount >= 5 { // Only check first 5 messages for performance
			break
		}
		for _, part := range msg.Parts {
			if text, ok := part.(llms.TextContent); ok {
				contentLower := strings.ToLower(text.Text)
				for _, word := range queryWords {
					if strings.Contains(contentLower, word) {
						score += 3 // Content matches are less weighted
					}
				}
			}
		}
		messageCount++
	}

	// Check filename
	fileName := filepath.Base(path)
	fileNameLower := strings.ToLower(fileName)
	for _, word := range queryWords {
		if strings.Contains(fileNameLower, word) {
			score += 8
		}
	}

	// Only include results with some match
	if score > 0 {
		result.Score = score
		return result, true
	}

	return HistorySearchResult{}, false
}

// FormatSearchResult formats a search result for display
func FormatSearchResult(r HistorySearchResult) string {
	var parts []string

	// Format date
	dateStr := r.Created.Format("2006-01-02 15:04")
	parts = append(parts, fmt.Sprintf("\033[36m%s\033[0m", dateStr))

	// Add description
	if r.Description != "" {
		desc := r.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		parts = append(parts, desc)
	}

	// Add topics
	if len(r.Topics) > 0 {
		parts = append(parts, fmt.Sprintf("\033[33m[%s]\033[0m", strings.Join(r.Topics, ", ")))
	}

	// Add tags
	if len(r.Tags) > 0 {
		tagStr := strings.Join(r.Tags, " #")
		parts = append(parts, fmt.Sprintf("\033[90m#%s\033[0m", tagStr))
	}

	// Add file path (dimmed)
	parts = append(parts, fmt.Sprintf("\033[38;5;240m%s\033[0m", filepath.Base(r.Path)))

	return strings.Join(parts, " ")
}