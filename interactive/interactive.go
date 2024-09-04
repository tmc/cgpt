package interactive

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chzyer/readline"
)

var pasteThreshold = 50 * time.Millisecond

type MultilineState int

const (
	MultilineNone MultilineState = iota
	MultilineActive
)

// Config defines the required parameters for creating a new interactive session.
type Config struct {
	Prompt         string
	AltPrompt      string
	HistoryFile    string
	ProcessFn      func(input string) error
	PasteThreshold time.Duration
}

type InteractiveSession struct {
	reader    *readline.Instance
	config    Config
	buffer    strings.Builder
	mlState   MultilineState
	lastInput time.Time
}

func NewInteractiveSession(cfg Config) (*InteractiveSession, error) {
	readlineConfig := &readline.Config{
		Prompt:            cfg.Prompt,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistoryFile:       cfg.HistoryFile,
		HistorySearchFold: true,
		AutoComplete:      readline.NewPrefixCompleter(),
	}

	reader, err := readline.NewEx(readlineConfig)
	if err != nil {
		return nil, err
	}

	session := &InteractiveSession{
		reader:    reader,
		config:    cfg,
		lastInput: time.Now(),
	}

	return session, nil
}

func (s *InteractiveSession) changePrompt(toAlt bool) {
	if toAlt {
		s.reader.SetPrompt(s.config.AltPrompt)
	} else {
		s.reader.SetPrompt(s.config.Prompt)
	}
	s.reader.Refresh()
}

func (s *InteractiveSession) Run() error {
	defer s.reader.Close()

	for {
		var line string
		var err error
		now := time.Now()
		timeDelta := now.Sub(s.lastInput)
		s.lastInput = now

		line, err = s.reader.Readline()

		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				return err
			}
			s.buffer.Reset()
			s.changePrompt(false)
			continue
		} else if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		// Check for Ctrl-X Ctrl-E sequence
		if line == "\x18\x05" {
			currentLine := s.buffer.String()
			editedLine, err := s.editInEditor(currentLine)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error editing in external editor: %v\n", err)
				continue
			}
			s.buffer.Reset()
			s.buffer.WriteString(editedLine)
			fmt.Print(editedLine) // Print the edited line to the console
			continue
		}

		s.buffer.WriteString(line)
		s.buffer.WriteString("\n")

		if s.mlState == MultilineNone {
			if s.shouldStartMultiline(line) {
				s.mlState = MultilineActive
				s.changePrompt(true)
				line = strings.TrimPrefix(line, `"""`)
			}
		} else {
			if s.shouldEndMultiline(line) {
				s.mlState = MultilineNone
				s.changePrompt(false)
				line = strings.TrimSuffix(line, `"""`)
			}
		}

		if timeDelta > s.config.PasteThreshold && s.isInputComplete() {
			input := s.buffer.String()
			if err := s.config.ProcessFn(input); err != nil {
				return err
			}
			s.buffer.Reset()
			s.changePrompt(false) // Back to normal prompt after completing multiline input
		}
	}

	return nil
}

func (s *InteractiveSession) readInput() (string, error) {
	line, err := s.reader.Readline()
	if s.shouldStartMultiline(line) {
		s.mlState = MultilineActive
		s.changePrompt(true)
		line = strings.TrimPrefix(line, `"""`)
	}
	if s.shouldEndMultiline(line) {
		s.mlState = MultilineNone
		s.changePrompt(false)
		line = strings.TrimSuffix(line, `"""`)
	}
	return line, err
}

func (s *InteractiveSession) readPastedInput() string {
	var buffer strings.Builder
	rl := bufio.NewReader(s.reader.Config.Stdin)
	for {
		line, _ := rl.ReadString('\n')
		if len(strings.TrimSpace(line)) == 0 {
			break
		}
		buffer.WriteString(strings.TrimSuffix(line, "\n") + "\n")
	}
	return buffer.String()
}

func (s *InteractiveSession) isInputComplete() bool {
	if s.mlState == MultilineActive {
		return false
	}
	input := s.buffer.String()
	return strings.HasSuffix(input, "\n\n")
}

func (s *InteractiveSession) shouldStartMultiline(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, `"""`) && (len(trimmed) == 3 || !strings.HasSuffix(trimmed, `"""`))
}

func (s *InteractiveSession) shouldEndMultiline(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasSuffix(trimmed, `"""`) && (len(trimmed) == 3 || strings.HasPrefix(trimmed, `"""`))
}

func (s *InteractiveSession) loadHistory() error {
	if s.config.HistoryFile == "" {
		return nil
	}
	file, err := os.Open(s.config.HistoryFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s.reader.SaveHistory(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (s *InteractiveSession) editInEditor(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default to vim if $EDITOR is not set
	}

	// Create a temporary file
	tmpfile, err := os.CreateTemp("", "cgpt_input_*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile.Name())

	// Write current content to the file
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}

	// Open the editor
	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Read the edited content
	editedContent, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", err
	}

	return string(editedContent), nil
}
