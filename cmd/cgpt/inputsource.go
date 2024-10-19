package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

type InputSource struct {
	Type  string // "stdin", "file", "string", or "arg"
	Value string
}

type InputHandler struct {
	Files   []string
	Strings []string
	Args    []string
	Stdin   io.Reader
}

// Process reads the set of inputs, this will block on stdin if it is included.
func (h *InputHandler) Process(ctx context.Context) ([]InputSource, error) {
	var sources []InputSource
	stdinContent, err := readStdin()
	if err != nil {
		return nil, fmt.Errorf("error reading from stdin: %w", err)
	}

	stdinUsed := false

	for _, file := range h.Files {
		if file == "-" {
			if stdinContent != "" {
				sources = append(sources, InputSource{Type: "stdin", Value: stdinContent})
				stdinUsed = true
			} else {
				sources = append(sources, InputSource{Type: "stdin", Value: ""})
			}
		} else {
			content, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("error reading file %s: %w", file, err)
			}
			sources = append(sources, InputSource{Type: "file", Value: string(content)})
		}
	}

	for _, s := range h.Strings {
		sources = append(sources, InputSource{Type: "string", Value: s})
	}

	for _, arg := range h.Args {
		sources = append(sources, InputSource{Type: "arg", Value: arg})
	}

	if !stdinUsed && stdinContent != "" {
		sources = append(sources, InputSource{Type: "stdin", Value: stdinContent})
	}

	return sources, nil
}

func readStdin() (string, error) {
	if !isReadingFromStdin() {
		return "", nil
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("error reading from stdin: %w", err)
	}

	return strings.TrimSpace(string(input)), nil
}

func isReadingFromStdin() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
