package main

import (
	"embed"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

func printBasicUsage() {
	fmt.Println(`
Examples:
	# Basic query about interpreting command output
	$ echo "how should I interpret the output of nvidia-smi?" | cgpt

	# Quick explanation request
	$ echo "explain plan 9 in one sentence" | cgpt

Advanced Examples:
	# Using a system prompt for a specific assistant role
	$ cgpt -s "You are a helpful programming assistant" -i "Write a Python function to calculate the Fibonacci sequence"

	# Code review using input from a file
	$ cat complex_code.py | cgpt -s "You are a code reviewer. Provide constructive feedback." -m "claude-3-5-sonnet-20240620"

	# Interactive session for creative writing
	$ cgpt -c -s "You are a creative writing assistant" # Start an interactive session for story writing

	# Show more advanced examples:
	$ cgpt --show-advanced-usage basic
	$ cgpt --show-advanced-usage all `)
}

func printAdvancedUsage(show string) {
	labels := strings.Split(show, ",")
	if len(labels) == 1 && labels[0] == "all" {
		labels = advancedUsageFiles
	}
	for _, label := range labels {
		content, ok := advancedUsageContents[label]
		if !ok {
			fmt.Fprintf(os.Stderr, "Unknown advanced usage example: %s\n", label)
			continue
		}
		fmt.Println(content)
	}
}

//go:embed usage-examples-*.md
var usageExamples embed.FS

var advancedUsageFiles, advancedUsageContents = additionalUsageFiles()

func additionalUsageFiles() ([]string, map[string]string) {
	files, _ := usageExamples.ReadDir(".")
	fileContents := make(map[string]string)

	for _, file := range files {
		name := strings.TrimPrefix(file.Name(), "usage-examples-")
		name = regexp.MustCompile(`^\d+-`).ReplaceAllString(name, "")
		name = strings.TrimSuffix(name, ".md")
		content, _ := usageExamples.ReadFile(file.Name())
		fileContents[name] = string(content)
	}

	fileNames := make([]string, 0, len(fileContents))
	for name := range fileContents {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	return fileNames, fileContents
}
