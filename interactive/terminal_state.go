package interactive

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// TerminalStateMachine maintains virtual terminal state and optimizes output
// It tracks what readline thinks is on screen vs what actually is
type TerminalStateMachine struct {
	realStdout io.Writer
	mu         sync.Mutex

	// Virtual screen state
	virtualLine string // What readline thinks is displayed
	actualLine  string // What's actually displayed

	// Operation detection
	clearCount  int
	lastWrite   time.Time
	rapidWrites int
	inPasteMode bool

	// Buffering
	opBuffer   *bytes.Buffer
	pendingOps []operation
}

type operation struct {
	typ     string // "clear", "write", "cr", etc.
	content []byte
	time    time.Time
}

// NewTerminalStateMachine creates an optimizing terminal state machine
func NewTerminalStateMachine() *TerminalStateMachine {
	return &TerminalStateMachine{
		realStdout: os.Stdout,
		opBuffer:   &bytes.Buffer{},
		pendingOps: make([]operation, 0, 100),
	}
}

// Write implements io.Writer with state machine optimization
func (t *TerminalStateMachine) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// Detect paste mode based on write frequency
	if now.Sub(t.lastWrite) < 3*time.Millisecond {
		t.rapidWrites++
		if t.rapidWrites > 5 && !t.inPasteMode {
			t.inPasteMode = true
			// During paste, we'll batch everything
		}
	} else if now.Sub(t.lastWrite) > 50*time.Millisecond {
		// Gap in writes - exit paste mode and flush
		if t.inPasteMode {
			t.flushPendingOps()
			t.inPasteMode = false
			t.rapidWrites = 0
		}
	}

	t.lastWrite = now

	// Parse the operation
	op := t.parseOperation(p)

	if t.inPasteMode {
		// During paste, collect operations
		t.pendingOps = append(t.pendingOps, op)

		// Don't write anything yet
		return len(p), nil
	}

	// Not in paste mode - apply optimization rules
	switch op.typ {
	case "clear":
		t.clearCount++
		// Skip excessive clears
		if t.clearCount > 2 && now.Sub(t.lastWrite) < 10*time.Millisecond {
			// Skip this clear
			return len(p), nil
		}
		t.virtualLine = ""

	case "write":
		content := string(op.content)
		t.virtualLine += content

		// Only write if different from actual
		if t.virtualLine != t.actualLine {
			t.realStdout.Write(op.content)
			t.actualLine = t.virtualLine
		}
		return len(p), nil

	case "cr":
		t.virtualLine = ""
		// Only send CR if needed
		if t.actualLine != "" {
			t.realStdout.Write([]byte("\r"))
		}
		return len(p), nil
	}

	// Default - pass through
	return t.realStdout.Write(p)
}

// parseOperation identifies what type of terminal operation this is
func (t *TerminalStateMachine) parseOperation(p []byte) operation {
	op := operation{
		content: p,
		time:    time.Now(),
	}

	// Check for escape sequences
	if bytes.Contains(p, []byte("\033[2K")) || bytes.Contains(p, []byte("\033[K")) {
		op.typ = "clear"
	} else if bytes.Contains(p, []byte("\033[J")) {
		op.typ = "clear-screen"
	} else if bytes.Equal(p, []byte("\r")) {
		op.typ = "cr"
	} else if bytes.Contains(p, []byte("\033[")) {
		op.typ = "escape"
	} else {
		op.typ = "write"
		// Extract just the printable content
		op.content = t.extractPrintable(p)
	}

	return op
}

// extractPrintable extracts only printable characters
func (t *TerminalStateMachine) extractPrintable(p []byte) []byte {
	var result bytes.Buffer

	for i := 0; i < len(p); i++ {
		// Skip escape sequences
		if i < len(p)-1 && p[i] == '\033' && p[i+1] == '[' {
			j := i + 2
			for j < len(p) && !isLetter(p[j]) {
				j++
			}
			if j < len(p) {
				i = j
				continue
			}
		}

		// Keep printable chars and newlines
		if (p[i] >= 32 && p[i] < 127) || p[i] == '\n' || p[i] == '\t' {
			result.WriteByte(p[i])
		}
	}

	return result.Bytes()
}

// flushPendingOps intelligently flushes pending operations
func (t *TerminalStateMachine) flushPendingOps() {
	if len(t.pendingOps) == 0 {
		return
	}

	// Analyze operations to determine final state
	var finalContent bytes.Buffer
	var lastClear int = -1

	// Find the last clear operation
	for i, op := range t.pendingOps {
		if op.typ == "clear" || op.typ == "cr" {
			lastClear = i
		}
	}

	// Start from the last clear (or beginning)
	startIdx := 0
	if lastClear >= 0 {
		startIdx = lastClear
		// Send one clear
		t.realStdout.Write([]byte("\r\033[K"))
	}

	// Accumulate all writes after the last clear
	for i := startIdx; i < len(t.pendingOps); i++ {
		if t.pendingOps[i].typ == "write" {
			finalContent.Write(t.pendingOps[i].content)
		}
	}

	// Write the final content
	if finalContent.Len() > 0 {
		t.realStdout.Write(finalContent.Bytes())
		t.actualLine = strings.TrimSpace(finalContent.String())
		t.virtualLine = t.actualLine
	}

	// Clear pending operations
	t.pendingOps = t.pendingOps[:0]
	t.clearCount = 0
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}
