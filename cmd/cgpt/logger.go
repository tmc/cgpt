package main

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Define ANSI color constants for better readability
const (
	grey          = "\033[38;5;240m"
	boldLightGrey = "\033[1;38;5;240m"
	red           = "\033[38;5;9m"
	yellow        = "\033[38;5;11m"
	green         = "\033[38;5;10m"
	blue          = "\033[38;5;14m"
	reset         = "\033[0m"
)

// fullLineColorLevelEncoder colors the entire output line based on log level
func fullLineColorLevelEncoder(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	var color string
	switch l {
	case zapcore.DebugLevel:
		color = grey
	case zapcore.InfoLevel:
		color = boldLightGrey
	case zapcore.WarnLevel:
		color = yellow
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		color = red
	default:
		color = reset
	}
	enc.AppendString(color + l.CapitalString())
}

// NewLogger creates a new zap SugaredLogger with the given configuration
func NewLogger(stderr io.Writer, verbose, debug bool) (*zap.SugaredLogger, error) {
	// Use stderr if provided, otherwise default to os.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	// 1. Base Config
	loggerCfg := zap.NewProductionConfig() // Start with Production for sane defaults
	loggerCfg.Encoding = "console"         // Crucial for console encoder features
	loggerCfg.EncoderConfig.TimeKey = ""   // Shorter key if desired, or "" to disable
	loggerCfg.EncoderConfig.LevelKey = "L" // Shorter key if desired, or "" to disable
	loggerCfg.EncoderConfig.NameKey = "N"
	loggerCfg.EncoderConfig.FunctionKey = ""
	loggerCfg.EncoderConfig.MessageKey = "M"
	loggerCfg.EncoderConfig.StacktraceKey = "S"
	loggerCfg.EncoderConfig.LineEnding = reset + zapcore.DefaultLineEnding // Add reset before newline
	loggerCfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder        // Or zapcore.ISO8601TimeEncoder etc.
	loggerCfg.EncoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	loggerCfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	loggerCfg.EncoderConfig.ConsoleSeparator = " " // Use space as separator

	// 2. Apply Full-Line Coloring
	loggerCfg.EncoderConfig.EncodeLevel = fullLineColorLevelEncoder // Use our custom encoder

	// 3. Set Initial Level (Default: Warn)
	loggerCfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	if verbose {
		loggerCfg.Level.SetLevel(zapcore.InfoLevel)
	}
	if debug {
		loggerCfg.Level.SetLevel(zapcore.DebugLevel)
		// Enable debug features in the config *before* building the logger
		loggerCfg.DisableStacktrace = false
		loggerCfg.EncoderConfig.CallerKey = "C" // Enable caller key for debug
	}

	// 4. Build the Logger
	stderrSyncer := zapcore.AddSync(stderr)

	// Create encoder using the configured EncoderConfig
	encoder := zapcore.NewConsoleEncoder(loggerCfg.EncoderConfig)

	// Create the core
	core := zapcore.NewCore(encoder, stderrSyncer, loggerCfg.Level)

	// Build logger options
	loggerOpts := []zap.Option{}

	// Only include caller location in Debug mode (its useful but noisy)
	if debug {
		loggerOpts = append(loggerOpts, zap.AddCaller(), zap.AddCallerSkip(1))
	}

	// Optional: Add hooks for log sampling or other features if needed
	// Optional: Add structured field defaults for all log entries
	// loggerOpts = append(loggerOpts, zap.Fields(zap.String("app", "cgpt")))

	// Build the logger and create sugared version
	logger := zap.New(core, loggerOpts...)
	return logger.Sugar(), nil
}
