package cgpt

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tmc/cgpt/interactive"
	"github.com/tmc/langchaingo/llms"
)

type CompletionService struct {
	cfg *Config

	model llms.Model

	payload *ChatCompletionPayload

	historyIn      io.Reader
	historyOutFile string
}

// NewCompletionService creates a new CompletionService with the given configuration.
func NewCompletionService(cfg *Config) (*CompletionService, error) {
	model, err := initializeModel(cfg.Backend, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize model: %w", err)
	}
	return &CompletionService{
		cfg:     cfg,
		model:   model,
		payload: newCompletionPayload(cfg),
	}, nil
}

// RunConfig is the configuration for the Run method.
type RunConfig struct {
	// Input is the input text to complete. If "-", read from stdin.
	Input string
	// Continuous will run the completion API in a loop, using the previous output as the input for the next request.
	Continuous bool
	// Stream will stream results as they come in.
	Stream bool

	// HistoryIn is the file to read history from.
	HistoryIn string
	// HistoryOut is the file to store history in.
	HistoryOut string

	// NCompletions is the number of completions to complete in a history-enabled context.
	NCompletions int

	// Verbose will enable verbose output.
	Verbose bool
}

// Run runs the completion service with the given configuration.
func (s *CompletionService) Run(ctx context.Context, runCfg RunConfig) error {
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
	err := s.loadHistory()
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
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

func (s *CompletionService) runOneShotCompletion(ctx context.Context, inputFile string) error {
	var (
		input io.Reader
		err   error
	)
	if inputFile == "-" {
		fmt.Printf("> ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		input = strings.NewReader(line)
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

func (s *CompletionService) runOneShotCompletionStreaming(ctx context.Context, inputFile string) error {
	var (
		input io.Reader = os.Stdin
		err   error
	)
	if inputFile != "-" {
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
	if err := s.saveHistory(); err != nil {
		return fmt.Errorf("failed to save history: %w", err)
	}
	return nil
}

// Enhanced function to run continuous completion mode.
func (s *CompletionService) runContinuousCompletion(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press ctrl+c to exit.")
	processFn := func(input string) error {
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
		return nil
	}

	sessionConfig := interactive.Config{
		Prompt:      ">>> ",
		AltPrompt:   "... ",
		HistoryFile: s.historyOutFile,
		ProcessFn:   processFn,
	}

	session, err := interactive.NewInteractiveSession(sessionConfig)
	if err != nil {
		return err
	}

	return session.Run()
}

// Enhanced function to run continuous streaming completion mode.
func (s *CompletionService) runContinuousCompletionStreaming(ctx context.Context) error {
	fmt.Fprintln(os.Stderr, "Running in continuous mode. Press ctrl+c to exit.")

	processFn := func(input string) error {
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
		return nil
	}

	sessionConfig := interactive.Config{
		Prompt:      ">>> ",
		AltPrompt:   "... ",
		HistoryFile: s.historyOutFile,
		ProcessFn:   processFn,
	}

	session, err := interactive.NewInteractiveSession(sessionConfig)
	if err != nil {
		return err
	}

	return session.Run()
}
