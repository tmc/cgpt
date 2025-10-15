package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/pflag"
)

//go:embed docs/usage_examples.md
var usageExamplesFile string

// Version information (can be overridden at build time with -ldflags)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

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

// printVersion displays version information
func printVersion() {
	fmt.Printf("cgpt version %s\n", version)
	if commit != "unknown" {
		fmt.Printf("  commit: %s\n", commit)
	}
	if buildDate != "unknown" {
		fmt.Printf("  built:  %s\n", buildDate)
	}
	fmt.Printf("  go:     %s\n", runtime.Version())
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func listSections() {
	fmt.Println("Available sections:")
	lines := strings.Split(usageExamplesFile, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			fmt.Println("-", strings.TrimPrefix(line, "## "))
		}
	}
}

// printEnhancedHelp prints a well-formatted help output with grouped flags
func printEnhancedHelp(programName string, fs *pflag.FlagSet) {
	fmt.Printf("cgpt - AI-powered command line assistant\n\n")

	fmt.Printf("USAGE:\n")
	fmt.Printf("  %s [flags] [input...]\n\n", programName)

	fmt.Printf("DESCRIPTION:\n")
	fmt.Printf("  cgpt is a versatile command-line tool for interacting with various AI models.\n")
	fmt.Printf("  It supports multiple input methods, conversation history, and advanced AI features.\n\n")

	printFlagsByCategory(fs)

	fmt.Printf("QUICK EXAMPLES:\n")
	fmt.Printf("  # Ask a simple question\n")
	fmt.Printf("  echo \"What is Go programming language?\" | cgpt\n\n")

	fmt.Printf("  # Interactive chat session\n")
	fmt.Printf("  cgpt -c\n\n")

	fmt.Printf("  # Analyze a file with custom instructions\n")
	fmt.Printf("  cgpt -f mycode.go -s \"You are a code reviewer. Find potential issues.\"\n\n")

	fmt.Printf("  # Continue previous conversation\n")
	fmt.Printf("  cgpt -C\n\n")

	fmt.Printf("ENVIRONMENT VARIABLES:\n")
	printEnvironmentVariables()

	fmt.Printf("\nMORE EXAMPLES:\n")
	fmt.Printf("  Use 'cgpt --examples' for more usage examples\n")
	fmt.Printf("  Use 'cgpt --show-advanced-usage all' for comprehensive examples\n")
}

// printFlagsByCategory prints flags organized by logical categories
func printFlagsByCategory(fs *pflag.FlagSet) {
	categories := []struct {
		title string
		flags []string
	}{
		{
			title: "INPUT/OUTPUT",
			flags: []string{"input", "file", "prefill", "continuous", "verbose", "debug"},
		},
		{
			title: "HISTORY & SESSIONS",
			flags: []string{"history-in", "history-out", "history", "continue", "completions"},
		},
		{
			title: "AI MODEL & BEHAVIOR",
			flags: []string{"backend", "model", "system-prompt", "max-tokens", "temperature"},
		},
		{
			title: "ADVANCED FEATURES",
			flags: []string{"prompt-caching", "thinking-mode", "thinking-budget", "usage", "show-reasoning", "interleaved-thinking"},
		},
		{
			title: "TECHNICAL OPTIONS",
			flags: []string{"completion-timeout", "max-retries", "retry-delay", "disable-retry"},
		},
		{
			title: "CONFIGURATION",
			flags: []string{"config", "help", "examples"},
		},
	}

	for _, category := range categories {
		fmt.Printf("%s:\n", category.title)
		for _, flagName := range category.flags {
			if flag := fs.Lookup(flagName); flag != nil {
				shorthand := ""
				if flag.Shorthand != "" {
					shorthand = fmt.Sprintf(", -%s", flag.Shorthand)
				}
				fmt.Printf("  --%s%s\n", flag.Name, shorthand)
				fmt.Printf("      %s\n", flag.Usage)
				if flag.DefValue != "" && flag.DefValue != "false" && flag.DefValue != "[]" {
					fmt.Printf("      (default: %s)\n", flag.DefValue)
				}
				fmt.Println()
			}
		}
	}
}

// printQuickExamples shows common usage patterns
func printQuickExamples() {
	fmt.Printf("cgpt - Quick Usage Examples\n\n")

	examples := []struct {
		title       string
		description string
		command     string
	}{
		{
			"Simple Query",
			"Ask the AI a direct question",
			`echo "Explain quantum computing in simple terms" | cgpt`,
		},
		{
			"Interactive Mode",
			"Start a conversation with the AI",
			`cgpt -c`,
		},
		{
			"File Analysis",
			"Analyze code or text files",
			`cgpt -f script.sh -s "Review this script for security issues"`,
		},
		{
			"Custom System Role",
			"Use the AI as a specialized assistant",
			`cgpt -s "You are a helpful DevOps engineer" -i "How do I deploy with Docker?"`,
		},
		{
			"Continue Session",
			"Resume your last conversation",
			`cgpt -C`,
		},
		{
			"Save Conversation",
			"Keep a record of your chat",
			`cgpt -H auto -c`,
		},
		{
			"Multiple Inputs",
			"Combine different input sources",
			`cgpt -i "Context: I'm learning Go" -f main.go -i "Please explain this code"`,
		},
		{
			"Usage Analytics",
			"See token usage and costs",
			`cgpt --usage -i "Summarize the benefits of renewable energy"`,
		},
		{
			"Different Models",
			"Try various AI models",
			`cgpt -m claude-haiku-3-20240307 -i "Quick fact about Mars"`,
		},
		{
			"Reasoning Mode",
			"Enable advanced thinking (Claude 4+)",
			`cgpt --thinking-mode high -i "Solve this logic puzzle: ..."`,
		},
	}

	for i, ex := range examples {
		fmt.Printf("%d. %s\n", i+1, ex.title)
		fmt.Printf("   %s\n", ex.description)
		fmt.Printf("   $ %s\n\n", ex.command)
	}

	fmt.Printf("TIPS:\n")
	fmt.Printf("• Use -c for interactive sessions\n")
	fmt.Printf("• Pipe output from other commands: ls -la | cgpt -s \"Explain this directory\"\n")
	fmt.Printf("• Set CGPT_API_KEY environment variable to avoid authentication prompts\n")
	fmt.Printf("• Use -H auto to automatically save conversation history\n")
	fmt.Printf("• Try different models with -m flag for various capabilities and costs\n\n")

	fmt.Printf("For comprehensive examples: cgpt --show-advanced-usage all\n")
}

// printEnvironmentVariables shows relevant environment variables
func printEnvironmentVariables() {
	envVars := []struct {
		name        string
		description string
		example     string
	}{
		{
			"CGPT_API_KEY",
			"API key for your chosen AI provider",
			"export CGPT_API_KEY=your_api_key_here",
		},
		{
			"CGPT_MODEL",
			"Default model to use (overridden by -m flag)",
			"export CGPT_MODEL=claude-haiku-3-20240307",
		},
		{
			"CGPT_BACKEND",
			"Default AI provider (overridden by -b flag)",
			"export CGPT_BACKEND=anthropic",
		},
		{
			"CGPT_CONFIG",
			"Path to configuration file (overridden by --config flag)",
			"export CGPT_CONFIG=~/.config/cgpt/config.yaml",
		},
	}

	for _, env := range envVars {
		fmt.Printf("  %s\n", env.name)
		fmt.Printf("      %s\n", env.description)
		fmt.Printf("      Example: %s\n\n", env.example)
	}
}
