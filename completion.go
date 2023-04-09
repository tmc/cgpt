package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// runOneShotCompletion runs the completion API once.
func runOneShotCompletion(ctx context.Context, cfg *Config) error {
	input := *flagInput
	if input == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		input = string(b)
	}

	// Currently, we don't support streaming for one-shot completions.
	cfg.Stream = false
	payload := newCompletionPayload(cfg)
	if cfg.SystemPrompt != "" {
		payload.addSystemMessage(cfg.SystemPrompt)
	}
	payload.addUserMessage(input)
	r, err := performCompletion(ctx, cfg.APIKey, payload)
	if err != nil {
		return err
	}
	if *flagJSON {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		return e.Encode(r)
	} else {
		fmt.Println(r.Choices[0].Message.Content)
	}
	return nil
}

// runContinuousCompletion runs the completion API in a loop, using the previous output as the input for the next request.
func runContinuousCompletion(ctx context.Context, cfg *Config) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press Ctrl+C to exit.")
	// read until two newlines using a scanner:
	scanner := bufio.NewScanner(os.Stdin)

	payload := newCompletionPayload(cfg)
	if cfg.SystemPrompt != "" {
		payload.addSystemMessage(cfg.SystemPrompt)
	}
	for {
		input, err := readUntilBlank(scanner, "> ")
		if err != nil {
			return err
		}
		payload.addUserMessage(input)
		r, err := performCompletion(ctx, cfg.APIKey, payload)
		if err != nil {
			return err
		}
		payload.addAssistantMessage(r.Choices[0].Message.Content)
		if *flagJSON {
			e := json.NewEncoder(os.Stdout)
			e.SetIndent("", "  ")
			return e.Encode(r)
		} else {
			fmt.Println(r.Choices[0].Message.Content)
			fmt.Println("")
		}
	}
}

// runContinuousCompletionStreaming runs the completion API in a loop, using the previous output as the input for the next request.
func runContinuousCompletionStreaming(ctx context.Context, cfg *Config) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press Ctrl+C to exit.")
	// read until two newlines using a scanner:
	scanner := bufio.NewScanner(os.Stdin)

	payload := newCompletionPayload(cfg)
	fmt.Println("system prompt: ", cfg.SystemPrompt)
	if cfg.SystemPrompt != "" {
		payload.addSystemMessage(cfg.SystemPrompt)
	}
	for {
		input, err := readUntilBlank(scanner, "> ")
		if err != nil {
			return err
		}
		payload.addUserMessage(input)
		streamPayloads, err := performCompletionStreaming(ctx, cfg.APIKey, payload)
		if err != nil {
			return err
		}
		content := strings.Builder{}
		for r := range streamPayloads {
			content.WriteString(r.Choices[0].Delta.Content)
			if *flagJSON {
				e := json.NewEncoder(os.Stdout)
				e.SetIndent("", "  ")
				return e.Encode(r)
			} else {
				fmt.Print(r.Choices[0].Delta.Content)
			}
		}
		payload.addAssistantMessage(content.String())
		fmt.Println()
	}
}

// readUntilBlank reads lines from the scanner until a blank line is encountered.
func readUntilBlank(s *bufio.Scanner, linePrompt string) (string, error) {
	var lines []string
	fmt.Print(linePrompt)
	for s.Scan() {
		line := s.Text()
		if line == "" {
			break
		}
		lines = append(lines, line)
		fmt.Print(linePrompt)
	}
	return strings.Join(lines, "\n"), s.Err()
}
