package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	grey          = "\033[38;5;240m"
	boldLightGrey = "\033[1;38;5;240m"
	red           = "\033[38;5;9m"
	yellow        = "\033[38;5;11m"
	green         = "\033[38;5;10m"
	blue          = "\033[38;5;14m"
	reset         = "\033[0m"
)

// MiddlewareHandler combines the standard handler (for source info) with our custom handler
type MiddlewareHandler struct {
	stdHandler    slog.Handler
	customHandler *CustomLogHandler
	attrs         []slog.Attr
}

func (h *MiddlewareHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.customHandler.Enabled(ctx, level)
}

func (h *MiddlewareHandler) Handle(ctx context.Context, r slog.Record) error {
	// Process the record through the standard handler to get source info
	// We'll discard its output but keep the attributes it adds
	stdRecord := r.Clone()
	if err := h.stdHandler.Handle(ctx, stdRecord); err != nil {
		return err
	}

	// Apply additional attributes from this handler
	for _, attr := range h.attrs {
		r.AddAttrs(attr)
	}

	// Pass the record with source information to the custom handler
	return h.customHandler.Handle(ctx, r)
}

func (h *MiddlewareHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MiddlewareHandler{
		stdHandler:    h.stdHandler.WithAttrs(attrs),
		customHandler: h.customHandler.WithAttrs(attrs).(*CustomLogHandler),
		attrs:         append(h.attrs, attrs...),
	}
}

func (h *MiddlewareHandler) WithGroup(name string) slog.Handler {
	return &MiddlewareHandler{
		stdHandler:    h.stdHandler.WithGroup(name),
		customHandler: h.customHandler.WithGroup(name).(*CustomLogHandler),
		attrs:         h.attrs,
	}
}

// CustomLogHandler implements slog.Handler with a custom format
type CustomLogHandler struct {
	level slog.Level
	out   io.Writer
	attrs []slog.Attr
}

func (h *CustomLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *CustomLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format the log entry
	color, levelStr := getLevelFormat(r.Level)

	// Extract source from record if available
	fileInfo := ""
	var sourceAttr slog.Value
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == slog.SourceKey {
			sourceAttr = attr.Value
			return false
		}
		return true
	})
	
	if sourceAttr.String() != "" {
		if src, ok := sourceAttr.Any().(*slog.Source); ok && src != nil {
			file := filepath.Base(src.File)
			fileInfo = fmt.Sprintf("%s:%d", file, src.Line)
		}
	}

	// Build the formatted log message
	var s strings.Builder
	s.WriteString(color)
	s.WriteString(levelStr)
	s.WriteString(" ")
	if fileInfo != "" {
		s.WriteString(fileInfo)
		s.WriteString(" ")
	}
	s.WriteString(r.Message)

	// Add relevant attributes
	r.Attrs(func(attr slog.Attr) bool {
		if isRelevantAttr(attr) {
			s.WriteString(" ")
			s.WriteString(attr.Key)
			s.WriteString("=")
			s.WriteString(attr.Value.String())
		}
		return true
	})

	s.WriteString(reset)
	s.WriteString("\n")

	_, err := fmt.Fprint(h.out, s.String())
	return err
}

func (h *CustomLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CustomLogHandler{
		level: h.level,
		out:   h.out,
		attrs: append(h.attrs, attrs...),
	}
}

func (h *CustomLogHandler) WithGroup(name string) slog.Handler {
	return h // We don't support groups for this simple handler
}

// Get color and text representation for a log level
func getLevelFormat(level slog.Level) (string, string) {
	switch level {
	case slog.LevelDebug:
		return grey, "DEBUG"
	case slog.LevelInfo:
		return boldLightGrey, "INFO"
	case slog.LevelWarn:
		return yellow, "WARN"
	case slog.LevelError:
		return red, "ERROR"
	default:
		return reset, level.String()
	}
}

// Check if attribute should be included in output
func isRelevantAttr(attr slog.Attr) bool {
	if attr.Key == "caller" && attr.Value.String() == "enabled" {
		return false
	}
	
	if attr.Key == slog.TimeKey || attr.Key == slog.LevelKey || attr.Key == slog.SourceKey {
		return false
	}
	
	return attr.Key == "continuous" || attr.Key == "streamOutput"
}

func NewLogger(stderr io.Writer, verbose, debug bool, logFilePath string, logLevel string) (*slog.Logger, error) {
	if stderr == nil {
		stderr = os.Stderr
	}

	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}
	if debug {
		level = slog.LevelDebug
	}
	
	// Override with explicit log-level if provided
	if logLevel != "" {
		switch strings.ToLower(logLevel) {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	// Create a wrapper handler using stdlib options to get proper source locations
	stdHandler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})

	// Create our custom handler for formatted output
	customHandler := &CustomLogHandler{
		level: level,
		out:   stderr,
	}

	// Create a middleware logging handler that will add source information
	// and then delegate to our custom handler for output
	handler := &MiddlewareHandler{
		stdHandler:    stdHandler,
		customHandler: customHandler,
	}

	logger := slog.New(handler)

	// If log file is specified, set up file logging
	var logFile *os.File
	if logFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		// For log files, use a standard text handler with source information
		fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		})

		// Instead of replacing the logger, just create a new one that writes to the file
		// Discard the stderr debug logs when a log file is specified
		if stderr == os.Stderr {
			// When using the log file, don't duplicate to stderr
			customHandler.out = io.Discard
		}
		
		// Add file logging only, don't replace the existing logger
		logger = slog.New(fileHandler)
	}

	if debug {
		logger = logger.With(slog.String("caller", "enabled"))
	}

	return logger, nil
}
