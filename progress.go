package cgpt

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ProgressIndicator provides visual feedback for long-running operations
type ProgressIndicator struct {
	writer        io.Writer
	message       string
	currentStep   string
	totalSteps    int
	currentStepNum int32
	isRunning     int32
	startTime     time.Time
	mu            sync.Mutex
	spinnerFrames []string
	spinnerIndex  int
	updateChan    chan progressUpdate
	stopChan      chan struct{}
}

type progressUpdate struct {
	step    string
	stepNum int
}

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator(w io.Writer, message string, totalSteps int) *ProgressIndicator {
	return &ProgressIndicator{
		writer:     w,
		message:    message,
		totalSteps: totalSteps,
		spinnerFrames: []string{
			"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
		},
		updateChan: make(chan progressUpdate, 10),
		stopChan:   make(chan struct{}),
	}
}

// Start begins the progress indicator
func (p *ProgressIndicator) Start() {
	if atomic.CompareAndSwapInt32(&p.isRunning, 0, 1) {
		p.startTime = time.Now()
		go p.run()
	}
}

// Stop stops the progress indicator
func (p *ProgressIndicator) Stop() {
	if atomic.CompareAndSwapInt32(&p.isRunning, 1, 0) {
		close(p.stopChan)
		// Clear the line
		fmt.Fprintf(p.writer, "\r\033[K")
	}
}

// UpdateStep updates the current step being processed
func (p *ProgressIndicator) UpdateStep(step string, stepNum int) {
	if atomic.LoadInt32(&p.isRunning) == 1 {
		select {
		case p.updateChan <- progressUpdate{step: step, stepNum: stepNum}:
		default:
			// Don't block if channel is full
		}
	}
}

// Complete marks the progress as complete with a final message
func (p *ProgressIndicator) Complete(message string) {
	p.Stop()
	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.writer, "\033[32m✓\033[0m %s \033[38;5;240m(%.1fs)\033[0m\n",
		message, elapsed.Seconds())
}

// Error marks the progress as failed with an error message
func (p *ProgressIndicator) Error(message string) {
	p.Stop()
	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.writer, "\033[31m✗\033[0m %s \033[38;5;240m(%.1fs)\033[0m\n",
		message, elapsed.Seconds())
}

func (p *ProgressIndicator) run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case update := <-p.updateChan:
			p.mu.Lock()
			p.currentStep = update.step
			atomic.StoreInt32(&p.currentStepNum, int32(update.stepNum))
			p.mu.Unlock()
		case <-ticker.C:
			p.draw()
		}
	}
}

func (p *ProgressIndicator) draw() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update spinner
	p.spinnerIndex = (p.spinnerIndex + 1) % len(p.spinnerFrames)
	spinner := p.spinnerFrames[p.spinnerIndex]

	// Calculate progress
	currentStep := atomic.LoadInt32(&p.currentStepNum)
	elapsed := time.Since(p.startTime)

	// Build progress bar if we have steps
	var progressBar string
	if p.totalSteps > 0 {
		width := 20
		filled := int(float64(currentStep) / float64(p.totalSteps) * float64(width))
		progressBar = fmt.Sprintf(" [%s%s] %d/%d",
			strings.Repeat("█", filled),
			strings.Repeat("░", width-filled),
			currentStep, p.totalSteps)
	}

	// Build status line
	status := fmt.Sprintf("\r\033[K\033[36m%s\033[0m %s%s",
		spinner, p.message, progressBar)

	if p.currentStep != "" {
		status += fmt.Sprintf(" - %s", p.currentStep)
	}

	// Add elapsed time
	status += fmt.Sprintf(" \033[38;5;240m(%.1fs)\033[0m", elapsed.Seconds())

	fmt.Fprint(p.writer, status)
}

// SimpleProgressBar provides a simple progress bar for known-size operations
type SimpleProgressBar struct {
	writer      io.Writer
	title       string
	total       int64
	current     int64
	width       int
	startTime   time.Time
	lastDraw    time.Time
	completed   bool
	mu          sync.Mutex
}

// NewSimpleProgressBar creates a simple progress bar
func NewSimpleProgressBar(w io.Writer, title string, total int64) *SimpleProgressBar {
	return &SimpleProgressBar{
		writer:    w,
		title:     title,
		total:     total,
		width:     40,
		startTime: time.Now(),
	}
}

// Update updates the progress bar with the current value
func (pb *SimpleProgressBar) Update(current int64) {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current = current

	// Throttle updates to avoid excessive drawing
	if time.Since(pb.lastDraw) < 50*time.Millisecond && current < pb.total {
		return
	}
	pb.lastDraw = time.Now()

	pb.draw()
}

// Complete marks the progress bar as complete
func (pb *SimpleProgressBar) Complete() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current = pb.total
	pb.completed = true
	pb.draw()
	fmt.Fprintln(pb.writer) // New line after completion
}

func (pb *SimpleProgressBar) draw() {
	if pb.total <= 0 {
		return
	}

	// Calculate percentage
	percentage := float64(pb.current) / float64(pb.total) * 100

	// Calculate filled width
	filled := int(float64(pb.current) / float64(pb.total) * float64(pb.width))

	// Calculate ETA
	elapsed := time.Since(pb.startTime)
	var eta string
	if pb.current > 0 && pb.current < pb.total {
		totalTime := elapsed * time.Duration(pb.total) / time.Duration(pb.current)
		remaining := totalTime - elapsed
		if remaining > 0 {
			eta = fmt.Sprintf(" ETA: %s", formatDuration(remaining))
		}
	}

	// Build progress bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)

	// Choose color based on completion
	color := "\033[36m" // Cyan for in-progress
	if pb.completed {
		color = "\033[32m" // Green for complete
	}

	// Draw the bar
	fmt.Fprintf(pb.writer, "\r\033[K%s%s\033[0m %s[%s] %.1f%%%s",
		color, pb.title, color, bar, percentage, eta)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		min := int(d.Minutes())
		sec := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", min, sec)
	}
	hour := int(d.Hours())
	min := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hour, min)
}

// ActivityIndicator shows activity without specific progress
type ActivityIndicator struct {
	writer    io.Writer
	message   string
	isRunning int32
	stopChan  chan struct{}
	frames    []string
}

// NewActivityIndicator creates a new activity indicator
func NewActivityIndicator(w io.Writer, message string) *ActivityIndicator {
	return &ActivityIndicator{
		writer:  w,
		message: message,
		stopChan: make(chan struct{}),
		frames: []string{
			"[    ]", "[=   ]", "[==  ]", "[=== ]", "[====]",
			"[ ===]", "[  ==]", "[   =]", "[    ]",
		},
	}
}

// Start begins the activity indicator
func (a *ActivityIndicator) Start() {
	if atomic.CompareAndSwapInt32(&a.isRunning, 0, 1) {
		go a.run()
	}
}

// Stop stops the activity indicator
func (a *ActivityIndicator) Stop() {
	if atomic.CompareAndSwapInt32(&a.isRunning, 1, 0) {
		close(a.stopChan)
		fmt.Fprintf(a.writer, "\r\033[K") // Clear the line
	}
}

func (a *ActivityIndicator) run() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	frameIndex := 0
	for {
		select {
		case <-a.stopChan:
			return
		case <-ticker.C:
			frame := a.frames[frameIndex]
			fmt.Fprintf(a.writer, "\r\033[K\033[36m%s\033[0m %s", frame, a.message)
			frameIndex = (frameIndex + 1) % len(a.frames)
		}
	}
}