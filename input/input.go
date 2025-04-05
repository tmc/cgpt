// Package input handles processing and combining various input sources for cgpt.
package input

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// SourceType identifies the origin of an input part.
type SourceType string

const (
	SourceStdin  SourceType = "stdin"
	SourceFile   SourceType = "file"
	SourceString SourceType = "string"
	SourceArg    SourceType = "arg"
)

// Processor manages combining input sources.
type Processor struct {
	files           []string
	strings         []string
	args            []string
	fileOrder       []string  // Order of -f/--file flags as specified on command line
	stringOrder     []string  // Order of -i/--input flags as specified on command line
	stdin           io.Reader // The initial stdin reader passed from main
	isStdinTerminal bool      // Was the original os.Stdin a TTY?
	forceContinuous bool      // Whether -c/--continuous flag was supplied
}

// NewProcessor creates a new input processor.
// Pass the result of term.IsTerminal(int(os.Stdin.Fd())) as isStdinTerminal.
// Set forceContinuous if the -c/--continuous flag was supplied.
func NewProcessor(files []string, strings []string, args []string, stdin io.Reader, isStdinTerminal bool, forceContinuous bool) *Processor {
	return &Processor{
		files:           files,
		strings:         strings,
		args:            args,
		fileOrder:       nil, // Will be set by WithFileOrder
		stringOrder:     nil, // Will be set by WithStringOrder
		stdin:           stdin,
		isStdinTerminal: isStdinTerminal,
		forceContinuous: forceContinuous,
	}
}

// WithFileOrder sets the order in which file flags were specified
func (p *Processor) WithFileOrder(fileOrder []string) *Processor {
	p.fileOrder = fileOrder
	return p
}

// WithStringOrder sets the order in which input string flags were specified
func (p *Processor) WithStringOrder(stringOrder []string) *Processor {
	p.stringOrder = stringOrder
	return p
}

// GetCombinedReader creates and returns a single io.Reader that concatenates
// all specified input sources, plus an optional warning message.
// If fileOrder and stringOrder are non-nil, uses command-line order of flags,
// otherwise follows order: Files (-f, including auto-added '-'), Strings (-i), Args.
// Piped stdin is automatically included as the first '-f -' source if not explicitly listed.
// If tryReattachTTY is true, the caller should attempt to reattach to /dev/tty after reading from the returned reader.
func (p *Processor) GetCombinedReader(ctx context.Context) (reader io.Reader, warningMsg string, tryReattachTTY bool, err error) {
	var readers []io.Reader
	stdinUsed := false

	// Map to track which files and strings we've already processed
	processedFiles := make(map[string]bool)
	processedStrings := make(map[string]bool)

	// --- Auto-include Piped Stdin Logic ---
	stdinIsExplicitlyUsed := false
	for _, f := range p.files {
		if f == "-" {
			stdinIsExplicitlyUsed = true
			break
		}
	}

	// If stdin is NOT a terminal (piped/redirected) AND '-' was not explicitly used
	autoincludeStdin := !p.isStdinTerminal && !stdinIsExplicitlyUsed && p.stdin != nil
	// --- End Auto-include ---

	// Handle auto-including stdin
	if autoincludeStdin {
		// Prepend "-" to p.files to ensure it's processed first
		p.files = append([]string{"-"}, p.files...)
	}

	// If we have ordered flags from command line, use that order
	if p.fileOrder != nil || p.stringOrder != nil {
		// First process in the order flags appeared on command line
		// Create a merged list of all inputs in the order they appeared
		type inputItem struct {
			kind  string // "file" or "string"
			value string
		}
		var orderedInputs []inputItem

		// Add file flags in order
		for _, f := range p.fileOrder {
			orderedInputs = append(orderedInputs, inputItem{kind: "file", value: f})
		}

		// Add string flags in order
		for _, s := range p.stringOrder {
			orderedInputs = append(orderedInputs, inputItem{kind: "string", value: s})
		}

		// If auto-including stdin, put it at the front
		if autoincludeStdin {
			orderedInputs = append([]inputItem{{kind: "file", value: "-"}}, orderedInputs...)
		}

		// Process in flag-appearance order
		for _, item := range orderedInputs {
			if item.kind == "file" {
				// Process file
				file := item.value
				processedFiles[file] = true

				if file == "-" {
					if p.stdin == nil {
						readers = append(readers, strings.NewReader(""))
					} else if !stdinUsed {
						readers = append(readers, p.stdin)
						stdinUsed = true
					} else {
						// Ignore subsequent explicit '-' if stdin already consumed
						readers = append(readers, strings.NewReader(""))
					}
				} else {
					f, fileErr := os.Open(file)
					if fileErr != nil {
						return nil, warningMsg, false, fmt.Errorf("opening file '%s': %w", file, fileErr)
					}
					readers = append(readers, f)
				}
			} else if item.kind == "string" {
				// Process string input
				s := item.value
				processedStrings[s] = true
				readers = append(readers, strings.NewReader(s))
			}
		}
	}

	// Now add any files that weren't in the ordered list
	for _, file := range p.files {
		if !processedFiles[file] {
			if file == "-" {
				if p.stdin == nil {
					readers = append(readers, strings.NewReader(""))
				} else if !stdinUsed {
					readers = append(readers, p.stdin)
					stdinUsed = true
				} else {
					// Ignore subsequent explicit '-' if stdin already consumed
					readers = append(readers, strings.NewReader(""))
				}
			} else {
				f, fileErr := os.Open(file)
				if fileErr != nil {
					return nil, warningMsg, false, fmt.Errorf("opening file '%s': %w", file, fileErr)
				}
				readers = append(readers, f)
			}
			processedFiles[file] = true
		}
	}

	// Add any strings that weren't in the ordered list
	for _, s := range p.strings {
		if !processedStrings[s] {
			readers = append(readers, strings.NewReader(s))
			processedStrings[s] = true
		}
	}

	// Add positional args
	if len(p.args) > 0 {
		readers = append(readers, strings.NewReader(strings.Join(p.args, " ")))
	}

	// Determine if we need to reattach to TTY
	// If continuous mode flag is explicitly set (-c) AND we used stdin that isn't a terminal,
	// we'll need to try to reattach to the terminal for interactive mode
	tryReattachTTY = p.forceContinuous && stdinUsed && !p.isStdinTerminal

	// Combine all readers
	reader = io.MultiReader(readers...)
	return reader, warningMsg, tryReattachTTY, nil
}

// GetInputReader is a compatibility function that creates an input reader
// from the given sources. This is meant to ease migration.
func GetInputReader(ctx context.Context, files []string, strings []string, args []string, stdin io.Reader) (io.Reader, error) {
	// Use the processor with default values for terminal status and continuous mode
	p := NewProcessor(files, strings, args, stdin, false, false)
	reader, _, _, err := p.GetCombinedReader(ctx)
	return reader, err
}
