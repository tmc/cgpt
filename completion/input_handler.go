package completion

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/tmc/cgpt/input"
)

// handleLegacyInput handles input from legacy RunOptions struct
func (s *Service) handleLegacyInput(ctx context.Context, runCfgInterface interface{}) error {
	// Try to extract:
	// - InputFiles []string
	// - InputStrings []string
	// - PositionalArgs []string
	// - Stdin io.Reader

	// We need to use reflection since we don't know the exact type
	val := reflect.ValueOf(runCfgInterface)

	// Handle pointer or interface
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Must be a struct
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %v", val.Kind())
	}

	// Extract the fields we need
	var inputFiles, inputStrings, positionalArgs []string
	var stdin io.Reader

	// Try to get InputFiles
	if field := val.FieldByName("InputFiles"); field.IsValid() {
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
			slice := make([]string, field.Len())
			for i := 0; i < field.Len(); i++ {
				slice[i] = field.Index(i).String()
			}
			inputFiles = slice
		}
	}

	// Try to get InputStrings
	if field := val.FieldByName("InputStrings"); field.IsValid() {
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
			slice := make([]string, field.Len())
			for i := 0; i < field.Len(); i++ {
				slice[i] = field.Index(i).String()
			}
			inputStrings = slice
		}
	}

	// Try to get PositionalArgs
	if field := val.FieldByName("PositionalArgs"); field.IsValid() {
		if field.Kind() == reflect.Slice && field.Type().Elem().Kind() == reflect.String {
			slice := make([]string, field.Len())
			for i := 0; i < field.Len(); i++ {
				slice[i] = field.Index(i).String()
			}
			positionalArgs = slice
		}
	}

	// Try to get Stdin
	if field := val.FieldByName("Stdin"); field.IsValid() {
		if !field.IsNil() {
			if reader, ok := field.Interface().(io.Reader); ok {
				stdin = reader
			}
		}
	}

	// Now use the extracted fields
	// Default values for terminal status and continuous mode
	p := input.NewProcessor(inputFiles, inputStrings, positionalArgs, stdin, false, false)
	inputReader, _, _, err := p.GetCombinedReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to read inputs: %w", err)
	}

	inputData, err := io.ReadAll(inputReader)
	if err != nil {
		return fmt.Errorf("failed to read inputs: %w", err)
	}

	if len(inputData) != 0 {
		s.payload.addUserMessage(string(inputData))
	}

	return nil
}

// This function has been moved to completion.go
