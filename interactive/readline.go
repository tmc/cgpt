package interactive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chzyer/readline"
	"golang.org/x/term"
)

const (
	bracketedPasteStart = "\x1b[200~"
	bracketedPasteEnd   = "\x1b[201~"
)

type ReadlineSession struct {
	rl     *readline.Instance
	config Config
	log    *slog.Logger

	// input / editing
	mu            sync.Mutex
	buffer        strings.Builder
	state         InteractiveState
	responseState atomic.Value
	multiline     bool
	pendingSubmit bool
	lastInput     string

	// key-sequence helpers

	// interrupt / cancel
	currentProcessCancel context.CancelFunc

	// bracketed paste
	inPasteMode      bool
	disableRedraw    bool
	lastPasteRedraw  time.Time
	accumulatedPaste strings.Builder

	// misc.
	interruptCount int
	lastCtrlCTime  time.Time
}

var _ Session = (*ReadlineSession)(nil)

/* -------------------------------------------------------------------- */
/*  SIMPLE HISTORY & QUIT                                               */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) GetHistoryFilename() string { return s.config.HistoryFile }

func (s *ReadlineSession) LoadHistory(string) error {
	s.log.Warn("LoadHistory not implemented for readline session")
	return nil
}

func (s *ReadlineSession) SaveHistory(string) error {
	s.log.Warn("SaveHistory not implemented for readline session")
	return nil
}

func (s *ReadlineSession) Quit() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.rl != nil && s.rl.Config.Stdout != nil {
		/* disable bracketed paste */
		fmt.Fprint(s.rl.Config.Stdout, "\x1b[?2004l")
	}
	if s.rl != nil {
		s.rl.Close()
	}
}

/* -------------------------------------------------------------------- */
/*  SMALL HELPERS                                                       */
/* -------------------------------------------------------------------- */

func formatByteSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
}

func ansiDimColor(t string) string { return "\x1b[90m" + t + "\x1b[0m" }

func expandTilde(p string) (string, error) {
	if p == "" || !strings.HasPrefix(p, "~") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~")), nil
}

/* -------------------------------------------------------------------- */
/*  CONSTRUCTOR                                                         */
/* -------------------------------------------------------------------- */

func NewSession(cfg Config) (Session, error) {
	log := cfg.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	log = log.WithGroup("readline")

	if cfg.SingleLineHint == "" {
		cfg.SingleLineHint = DefaultSingleLineHint
	}
	if cfg.MultiLineHint == "" {
		cfg.MultiLineHint = DefaultMultiLineHint
	}

	if cfg.Prompt != "" && !strings.HasSuffix(cfg.Prompt, " ") {
		cfg.Prompt += " "
	}
	if cfg.AltPrompt != "" && !strings.HasSuffix(cfg.AltPrompt, " ") {
		cfg.AltPrompt += " "
	}

	hist, err := expandTilde(cfg.HistoryFile)
	if err == nil {
		cfg.HistoryFile = hist
	}

	s := &ReadlineSession{
		config: cfg,
		log:    log,
		state:  StateSingleLine,
	}
	s.responseState.Store(ResponseStateReady)

	/* ------------------------------------------------------------ */
	/*  painter & listener (kept verbatim from your version)        */
	/* ------------------------------------------------------------ */

	listener := s.createListener()
	painter := PainterFunc(func(line []rune, pos int) []rune {
		if s.responseState.Load().(ResponseState) == ResponseStateStreaming || s.inPasteMode {
			return line
		}
		if len(line) == 0 && s.buffer.Len() == 0 {
			return line
		}
		return line
	})

	isTTY := func() bool {
		if f, ok := cfg.Stdin.(*os.File); ok {
			return term.IsTerminal(int(f.Fd()))
		}
		return term.IsTerminal(int(os.Stdout.Fd()))
	}

	rlCfg := &readline.Config{
		Prompt:              cfg.Prompt,
		InterruptPrompt:     "^C",
		EOFPrompt:           "exit",
		HistoryFile:         cfg.HistoryFile,
		HistoryLimit:        10_000,
		HistorySearchFold:   true,
		Listener:            listener,
		Painter:             painter,
		ForceUseInteractive: true,
		FuncIsTerminal:      isTTY,
	}
	if cfg.Stdin != nil {
		rlCfg.Stdin = cfg.Stdin
	}
	if cfg.Stdout != nil {
		rlCfg.Stdout = cfg.Stdout
	}
	if cfg.Stderr != nil {
		rlCfg.Stderr = cfg.Stderr
	}

	r, err := readline.NewEx(rlCfg)
	if err != nil {
		return nil, fmt.Errorf("init readline: %w", err)
	}
	s.rl = r

	if rlCfg.Stdout != nil && isTTY() {
		fmt.Fprint(rlCfg.Stdout, "\x1b[?2004h") // enable bracketed paste
	}

	log.Info("readline session initialised")
	return s, nil
}

/* -------------------------------------------------------------------- */
/*  STATE-AWARE PROMPT                                                  */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) getPrompt() string {
	st := s.responseState.Load().(ResponseState)
	// Don't show prompt during any processing state
	if st == ResponseStateStreaming || st == ResponseStateSubmitted || st == ResponseStateSubmitting {
		return ""
	}
	if s.multiline {
		return s.config.AltPrompt
	}
	p := s.config.Prompt
	if s.pendingSubmit {
		p = strings.TrimSuffix(p, " ")
		return p + ansiDimColor("↵")
	}
	if p != "" && !strings.HasSuffix(p, " ") {
		p += " "
	}
	return p
}

/* -------------------------------------------------------------------- */
/*  RESPONSE STATE TRANSITIONS                                          */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) SetResponseState(st ResponseState) {
	prev := s.responseState.Load().(ResponseState)
	s.responseState.Store(st)

	if st == ResponseStateReady || st == ResponseStateSInterrupted {
		s.mu.Lock()
		if s.rl != nil {
			if prev.IsProcessing() && st == ResponseStateReady {
				fmt.Fprint(s.rl.Config.Stderr, "\r\033[K")
			}
			s.rl.SetPrompt(s.getPrompt())
			s.rl.Clean()
			s.rl.Refresh()
		}
		s.mu.Unlock()
	}
}

/* -------------------------------------------------------------------- */
/*  ADD RESPONSE PART (UNCHANGED)                                       */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) AddResponsePart(part string) {
	if st := s.responseState.Load().(ResponseState); st == ResponseStateSInterrupted || st == ResponseStateReady {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rl == nil {
		fmt.Print(part)
		return
	}
	fmt.Fprint(s.rl.Config.Stdout, part)
}

/* -------------------------------------------------------------------- */
/*  BRACKETED-PASTE HANDLER (identical to yours)                        */
/* -------------------------------------------------------------------- */

const pasteRedrawInterval = 500 * time.Millisecond
const pasteSizeReportThreshold = 100

func (s *ReadlineSession) handleBracketedPaste(line []rune, key rune) (bool, []rune, int) {
	str := string(line)

	if idx := strings.Index(str, bracketedPasteStart); idx != -1 {
		s.inPasteMode, s.disableRedraw = true, true
		s.lastPasteRedraw = time.Now()
		s.accumulatedPaste.Reset()

		before := str[:idx]
		after := str[idx+len(bracketedPasteStart):]

		if end := strings.Index(after, bracketedPasteEnd); end != -1 {
			s.inPasteMode, s.disableRedraw = false, false
			content := after[:end]
			rest := after[end+len(bracketedPasteEnd):]
			s.accumulatedPaste.WriteString(content)

			if n := s.accumulatedPaste.Len(); n > pasteSizeReportThreshold {
				fmt.Fprintf(s.rl.Config.Stderr, "\r%s\n", ansiDimColor("[Pasted "+formatByteSize(n)+"]"))
			}
			cl := before + content + rest
			return true, []rune(cl), len([]rune(cl))
		}

		s.accumulatedPaste.WriteString(after)
		cl := before + after
		return true, []rune(cl), len([]rune(cl))
	}

	if s.inPasteMode {
		if end := strings.Index(str, bracketedPasteEnd); end != -1 {
			s.inPasteMode, s.disableRedraw = false, false
			before := str[:end]
			after := str[end+len(bracketedPasteEnd):]
			s.accumulatedPaste.WriteString(before)

			if n := s.accumulatedPaste.Len(); n > pasteSizeReportThreshold {
				fmt.Fprintf(s.rl.Config.Stderr, "\r%s\n", ansiDimColor("[Pasted "+formatByteSize(n)+"]"))
			}
			cl := before + after
			return true, []rune(cl), len([]rune(cl))
		}

		s.accumulatedPaste.WriteString(str)
		if s.lastPasteRedraw.IsZero() || time.Since(s.lastPasteRedraw) > pasteRedrawInterval {
			s.lastPasteRedraw, s.disableRedraw = time.Now(), false
			return true, line, len(line)
		}
		s.disableRedraw = true
		return true, line, len(line)
	}

	return false, line, len(line)
}

/* -------------------------------------------------------------------- */
/*  LISTENER (unchanged except no CleanBeforeWrite)                     */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) createListener() readline.Listener {
	return readline.FuncListener(func(line []rune, pos int, key rune) ([]rune, int, bool) {
		s.mu.Lock()
		defer s.mu.Unlock()

		if isPaste, mod, mpos := s.handleBracketedPaste(line, key); isPaste {
			if s.disableRedraw {
				return mod, mpos, true
			}
			return mod, mpos, false
		}

		/* Up-arrow history replay when line is empty */
		if key == readline.CharPrev && len(line) == 0 && s.buffer.Len() == 0 && s.lastInput != "" {
			return []rune(s.lastInput), len([]rune(s.lastInput)), false
		}

		/* External editor feature removed */

		return line, pos, false
	})
}

/* -------------------------------------------------------------------- */
/*  RUN LOOP (verbatim from your file, only CleanBeforeWrite → Clean)   */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) Run(ctx context.Context) error {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.currentProcessCancel != nil {
			s.currentProcessCancel()
			s.currentProcessCancel = nil
		}
		if s.rl != nil && s.rl.Config.Stdout != nil {
			fmt.Fprint(s.rl.Config.Stdout, "\x1b[?2004l")
		}
		if s.rl != nil {
			s.rl.Close()
			s.rl = nil
		}
	}()

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			s.SetResponseState(ResponseStateSInterrupted)
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.currentProcessCancel != nil {
				s.currentProcessCancel()
				s.currentProcessCancel = nil
			}
			if s.rl != nil {
				s.rl.Close()
			}
		case <-done:
		}
	}()
	defer close(done)

	inTriple := false
	submit := false

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		s.mu.Lock()
		if s.rl == nil {
			s.mu.Unlock()
			return errors.New("readline closed")
		}
		s.rl.SetPrompt(s.getPrompt())
		s.mu.Unlock()

		line, err := s.rl.Readline()

		if ctx.Err() != nil {
			return ctx.Err()
		}

		s.mu.Lock()

		/* ------------- Ctrl-C ------------- */
		if errors.Is(err, readline.ErrInterrupt) {
			st := s.responseState.Load().(ResponseState)
			if !st.IsProcessing() {
				if len(line) > 0 || s.buffer.Len() > 0 || s.pendingSubmit {
					fmt.Fprint(s.rl.Config.Stderr, "\r"+ansiDimColor("Input cleared")+"\r")
					s.buffer.Reset()
					inTriple, s.multiline, s.pendingSubmit = false, false, false
									s.mu.Unlock()
					continue
				}
				fmt.Fprint(s.rl.Config.Stderr, "\r"+ansiDimColor("Exiting...")+"\r")
				s.mu.Unlock()
				return ErrInterrupted
			}

			s.SetResponseState(ResponseStateSInterrupted)
			if s.currentProcessCancel != nil {
				s.currentProcessCancel()
				s.currentProcessCancel = nil
			}
			fmt.Fprintf(s.rl.Config.Stdout, "%s\n", ansiDimColor(" [Interrupted]"))
			s.buffer.Reset()
			inTriple, s.multiline, s.pendingSubmit = false, false, false
					s.rl.SetPrompt(s.getPrompt())
			s.rl.Clean()
			s.rl.Refresh()
			s.mu.Unlock()
			continue
		}

		/* ------------- Ctrl-D / EOF ------------- */
		if errors.Is(err, io.EOF) {
			if s.buffer.Len() > 0 || len(line) > 0 {
				if s.buffer.Len() == 0 && len(line) > 0 {
					s.buffer.WriteString(line)
				}
				submit = true
			} else {
				fmt.Fprint(s.rl.Config.Stderr, "\r"+ansiDimColor("Exiting...")+"\r")
				s.mu.Unlock()
				return io.EOF
			}
		} else if err != nil {
			if ctx.Err() != nil {
				s.mu.Unlock()
				return ctx.Err()
			}
			s.mu.Unlock()
			return fmt.Errorf("readline: %w", err)
		}

		/* ------------- normal line handling ------------- */

		trim := strings.TrimSpace(line)
		if trim == "\"\"\"" { // toggle triple-quote
			if inTriple {
				inTriple, s.multiline, s.pendingSubmit, submit = false, false, false, true
			} else {
				if s.buffer.Len() > 0 {
					s.buffer.Reset()
				}
				inTriple, s.multiline, s.pendingSubmit = true, true, false
			}
			s.mu.Unlock()
			if submit {
				goto SUBMIT
			}
			continue
		}

		if len(line) == 0 && !inTriple {
			if s.pendingSubmit {
				submit, s.pendingSubmit = true, false
			} else if s.multiline && s.buffer.Len() > 0 {
				submit, s.multiline = true, false
			} else {
				s.mu.Unlock()
				continue
			}
		} else {
			if !inTriple && (trim == "exit" || trim == "quit") {
				fmt.Fprint(s.rl.Config.Stderr, "\r"+ansiDimColor("Exiting...")+"\r")
				s.mu.Unlock()
				return io.EOF
			}
			if s.buffer.Len() > 0 {
				s.buffer.WriteString("\n")
			}
			s.buffer.WriteString(line)

			if !inTriple {
				s.pendingSubmit, s.multiline = true, false
			}
		}

	SUBMIT:
		if submit {
			submit = false
			s.multiline, s.pendingSubmit = false, false
			input := s.buffer.String()
			s.buffer.Reset()

			if strings.TrimSpace(input) != "" {
				s.SetResponseState(ResponseStateSubmitting)

				respCtx, cancel := context.WithCancel(ctx)
				s.currentProcessCancel = cancel

				go func(in string) {
					defer func() {
						s.mu.Lock()
						defer s.mu.Unlock()
						if s.currentProcessCancel != nil {
							s.currentProcessCancel = nil
						}
					}()

					err := s.config.ProcessFn(respCtx, in)
					final := ResponseStateReady

					switch {
					case errors.Is(err, context.Canceled), errors.Is(err, ErrInterrupted):
						final = ResponseStateSInterrupted
					case err == nil:
						func() {
							s.mu.Lock()
							defer s.mu.Unlock()
							s.lastInput = in
							_ = s.rl.SaveHistory(in)
						}()
					case errors.Is(err, ErrEmptyInput):
						// ignore
					default:
						fmt.Fprintf(s.rl.Config.Stderr, "Processing error: %v\n", err)
						final = ResponseStateError
					}
					s.SetResponseState(final)
				}(input)
			}
			inTriple = false
		}
		s.mu.Unlock()
	}
}

/* -------------------------------------------------------------------- */
/*  HISTORY ACCESSOR (unchanged)                                        */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) GetHistory() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rl != nil {
		return s.config.ConversationHistory
	}
	return nil
}

/* -------------------------------------------------------------------- */
/*  PROMPT REDRAW (Clean+Refresh)                                       */
/* -------------------------------------------------------------------- */

func (s *ReadlineSession) redrawPrompt() {
	if s.rl == nil {
		return
	}
	s.rl.Clean()
	s.rl.Refresh()
}


/* -------------------------------------------------------------------- */
/*  PainterFunc adapter                                                 */
/* -------------------------------------------------------------------- */

type PainterFunc func(line []rune, pos int) []rune

func (p PainterFunc) Paint(line []rune, pos int) []rune { return p(line, pos) }
