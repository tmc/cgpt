package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
)

type CompletionService struct {
	cfg *Config

	model llms.Model

	payload *ChatCompletionPayload

	historyIn      io.Reader
	historyOutFile string
}

func NewCompletionService(cfg *Config) (*CompletionService, error) {
	model, err := initializeModel(cfg.Backend, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}
	s := &CompletionService{
		cfg:     cfg,
		model:   model,
		payload: newCompletionPayload(cfg),
	}
	return s, nil
}

func (s *CompletionService) run(ctx context.Context, runCfg runConfig) error {
	if err := s.handleHistory(runCfg.HistoryIn, runCfg.HistoryOut); err != nil {
		log.Println("failed to handle history:", err)
	}
	if !s.loadedWithHistory() && s.cfg.SystemPrompt != "" {
		s.payload.addSystemMessage(s.cfg.SystemPrompt)
	}
	if runCfg.Continuous {
		if runCfg.Stream {
			return s.runContinuousCompletionStreaming(ctx)
		} else {
			return s.runContinuousCompletion(ctx)
		}
	}
	if runCfg.NCompletions > 0 && s.loadedWithHistory() {
		return s.runNCompletions(ctx, runCfg.NCompletions)
	}
	if runCfg.Stream {
		return s.runOneShotCompletionStreaming(ctx, runCfg.Input)
	}
	return s.runOneShotCompletion(ctx, runCfg.Input)
}

func (s *CompletionService) loadedWithHistory() bool {
	return s.historyIn != nil
}

func (s *CompletionService) handleHistory(historyIn, historyOut string) error {
	s.historyOutFile = historyOut
	if historyIn != "" {
		f, err := os.Open(historyIn)
		if err != nil {
			return fmt.Errorf("failed to open history file %q: %w", historyIn, err)
		}
		s.historyIn = f
		defer f.Close()
	}
	loadErr := s.loadHistory()
	if loadErr == nil {
		if err := s.saveHistory(); err != nil {
			return fmt.Errorf("failed to save history: %w", err)
		}
	}
	return loadErr
}

func (s *CompletionService) runNCompletions(ctx context.Context, n int) error {
	fmt.Println("Running", n, "completions")

	for i := 0; i < n; i++ {
		in := s.getLastUserMessage()
		if err := s.runOneCompletion(ctx, strings.NewReader(in)); err != nil {
			return err
		}
	}
	return nil
}

func (s *CompletionService) getLastUserMessage() string {
	// TODO(tmc): user msg
	if len(s.payload.Messages) == 0 {
		return ""
	}
	last := s.payload.Messages[len(s.payload.Messages)-1]
	var parts []string
	for _, m := range last.Parts {
		parts = append(parts, fmt.Sprint(m))
	}

	return strings.Join(parts, "\n")
}

// runOneCompletion runs the completion API once.
func (s *CompletionService) runOneCompletion(ctx context.Context, input io.Reader) error {
	b, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	contents := string(b)

	// Currently, we don't support streaming for these completions.
	s.payload.Stream = false
	s.payload.addUserMessage(contents)
	r, err := s.PerformCompletion(ctx, s.payload)
	if err != nil {
		return err
	}
	fmt.Println(r)
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// runOneShotCompletion runs the completion API once.
func (s *CompletionService) runOneShotCompletion(ctx context.Context, inputFile string) error {
	// TODO: exit gracefully if no input is provided within a certain time period.
	var (
		input io.Reader
		err   error
	)
	if inputFile == "-" {
		input = os.Stdin
	} else {
		input, err = os.Open(inputFile)
		if err != nil {
			return fmt.Errorf("failed to open input file %q: %w", inputFile, err)
		}
	}
	b, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	contents := string(b)

	s.payload.Stream = false
	s.payload.addUserMessage(contents)
	response, err := s.PerformCompletion(ctx, s.payload)
	if err != nil {
		return fmt.Errorf("failed to perform completion: %w", err)
	}
	fmt.Println(response)
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// runOneShotCompletion runs the completion API once.
func (s *CompletionService) runOneShotCompletionStreaming(ctx context.Context, inputFile string) error {
	// TODO: exit gracefully if no input is provided within a certain time period.
	var (
		input             io.Reader
		err               error
		shouldShowSpinner = false
	)
	if inputFile == "-" {
		input = os.Stdin
		shouldShowSpinner = stdinAppearsToBeTTY() && !*flagStream
	} else {
		input, err = os.Open(inputFile)
		if err != nil {
			return fmt.Errorf("failed to open input file %q: %w", inputFile, err)
		}
	}
	b, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	contents := string(b)

	s.payload.Stream = true
	s.payload.addUserMessage(contents)
	streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, shouldShowSpinner)
	if err != nil {
		return err
	}
	content := strings.Builder{}
	for r := range streamPayloads {
		content.WriteString(r)
		fmt.Print(r)
	}
	s.payload.addAssistantMessage(content.String())
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// runContinuousCompletion runs the completion API in a loop, using the previous output as the input for the next request.
func (s *CompletionService) runContinuousCompletion(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press Ctrl+C to exit.")
	// read until two newlines using a scanner:
	scanner := bufio.NewScanner(os.Stdin)

	for {
		input, err := readUntilBlank(scanner, "> ")
		if err != nil {
			return err
		}
		s.payload.addUserMessage(input)
		response, err := s.PerformCompletion(ctx, s.payload)
		if err != nil {
			return err
		}
		s.payload.addAssistantMessage(response)
		fmt.Println(response)
		fmt.Println()
		if err := s.saveHistory(); err != nil {
			return fmt.Errorf("failed to save history: %w", err)
		}
	}
}

// runContinuousCompletionStreaming runs the completion API in a loop, using the previous output as the input for the next request.
func (s *CompletionService) runContinuousCompletionStreaming(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press Ctrl+C to exit.")
	// read until two newlines using a scanner:
	scanner := bufio.NewScanner(os.Stdin)

	for {
		input, err := readUntilBlank(scanner, "> ")
		if err != nil {
			return err
		}
		s.payload.addUserMessage(input)
		streamPayloads, err := s.PerformCompletionStreaming(ctx, s.payload, true)
		if err != nil {
			return err
		}
		content := strings.Builder{}
		for r := range streamPayloads {
			content.WriteString(r)
			fmt.Print(r)
		}
		s.payload.addAssistantMessage(content.String())
		fmt.Println()

		if err := s.saveHistory(); err != nil {
			return fmt.Errorf("failed to save history: %w", err)
		}
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

func stdinAppearsToBeTTY() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}
