package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
)

//go:embed docs/usage_examples.md
var usageExamplesFile string

func printBasicUsage() {
	fmt.Println()
	fmt.Println(extractSection("Basic Usage"))
}

func printAdvancedUsage(show string) {
	if show == "all" {
		fmt.Println(usageExamplesFile)
		return
	}

	sections := strings.Split(show, ",")
	for _, section := range sections {
		content := extractSection(strings.TrimSpace(section))
		if content == "" {
			fmt.Fprintf(os.Stderr, "Unknown section: %s\n", section)
			continue
		}
		fmt.Println(content)
	}
}

func extractSection(sectionName string) string {
	lines := strings.Split(usageExamplesFile, "\n")
	inSection := false
	var sectionContent []string

	for _, line := range lines {
		// make case insensitive:
		if strings.HasPrefix(strings.ToLower(line), "## "+strings.ToLower(sectionName)) {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if inSection {
			sectionContent = append(sectionContent, line)
		}
	}

	return strings.TrimSpace(strings.Join(sectionContent, "\n"))
}
