package cgpt

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSecurityValidator_ValidateCommand(t *testing.T) {
	policy := &SecurityPolicy{
		AllowNetworkAccess:  false,
		AllowFileSystem:     true,
		MaxExecutionTime:    30 * time.Second,
		MaxMemoryUsage:      100 * 1024 * 1024,
		RestrictedPaths:     []string{"/etc", "/sys", "/proc"},
	}

	validator := NewSecurityValidator(policy)

	tests := []struct {
		name        string
		hook        Hook
		expectError bool
		errorMsg    string
	}{
		{
			name: "safe command",
			hook: Hook{
				Command: "/usr/bin/echo",
				Args:    []string{"hello"},
			},
			expectError: false,
		},
		{
			name: "dangerous command - rm",
			hook: Hook{
				Command: "rm",
				Args:    []string{"-rf", "/"},
			},
			expectError: true,
			errorMsg:    "dangerous command blocked",
		},
		{
			name: "dangerous command - sudo",
			hook: Hook{
				Command: "sudo",
				Args:    []string{"rm", "-rf", "/"},
			},
			expectError: true,
			errorMsg:    "dangerous command blocked",
		},
		{
			name: "shell metacharacters in command",
			hook: Hook{
				Command: "/bin/sh -c 'rm -rf /'",
			},
			expectError: true,
			errorMsg:    "shell metacharacters",
		},
		{
			name: "restricted path in arguments",
			hook: Hook{
				Command: "/bin/cat",
				Args:    []string{"/etc/passwd"},
			},
			expectError: true,
			errorMsg:    "restricted path blocked",
		},
		{
			name: "shell metacharacters in arguments",
			hook: Hook{
				Command: "/bin/echo",
				Args:    []string{"$(rm -rf /)"},
			},
			expectError: true,
			errorMsg:    "shell metacharacters",
		},
		{
			name: "invalid working directory",
			hook: Hook{
				Command:    "/bin/echo",
				Args:       []string{"test"},
				WorkingDir: "/etc",
			},
			expectError: true,
			errorMsg:    "restricted path",
		},
		{
			name: "dangerous environment variable",
			hook: Hook{
				Command: "/bin/echo",
				Environment: map[string]string{
					"LD_PRELOAD": "/evil/library.so",
				},
			},
			expectError: true,
			errorMsg:    "dangerous environment variable",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validator.ValidateCommand(test.hook)

			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if test.expectError && err != nil && test.errorMsg != "" {
				if !strings.Contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", test.errorMsg, err)
				}
			}
		})
	}
}

func TestContainsShellMetacharacters(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"safe_string", false},
		{"safe string", false},
		{"safe-string", false},
		{"safe_string_123", false},
		{"/path/to/file", false},
		{"string;dangerous", true},
		{"string && dangerous", true},
		{"string | dangerous", true},
		{"$(dangerous)", true},
		{"`dangerous`", true},
		{"string > file", true},
		{"string < file", true},
		{"string{dangerous}", true},
		{"string[dangerous]", true},
		{"string*pattern", true},
		{"string?pattern", true},
		{"string\\escape", true},
		{"string'quote'", true},
		{"string\"quote\"", true},
		{"string~expansion", true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := containsShellMetacharacters(test.input)
			if result != test.expected {
				t.Errorf("Expected %v for input '%s', got %v", test.expected, test.input, result)
			}
		})
	}
}

func TestSandboxedExecutor_ExecuteHookSandboxed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sandboxed execution test in short mode")
	}

	policy := &SecurityPolicy{
		AllowNetworkAccess:  false,
		AllowFileSystem:     true,
		MaxExecutionTime:    5 * time.Second,
		MaxMemoryUsage:      50 * 1024 * 1024, // 50MB
		RestrictedPaths:     []string{"/etc", "/sys", "/proc"},
	}

	executor := NewSandboxedExecutor(policy)

	// Create a safe test hook
	hook := Hook{
		Name:    "test-hook",
		Command: "/bin/echo",
		Args:    []string{"Hello, World!"},
		Timeout: 5 * time.Second,
	}

	ctx := &HookContext{
		Event:     EventPreCompletion,
		Timestamp: time.Now(),
		Config:    &Config{Backend: "test"},
	}

	result, err := executor.ExecuteHookSandboxed(context.Background(), hook, ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !result.Success {
		t.Errorf("Hook should have succeeded: %s", result.Error)
	}

	if !strings.Contains(result.Output, "Hello, World!") {
		t.Errorf("Expected output to contain 'Hello, World!', got: %s", result.Output)
	}

	if result.Duration == 0 {
		t.Error("Duration should be recorded")
	}
}

func TestSandboxedExecutor_SecurityValidationFailure(t *testing.T) {
	policy := &SecurityPolicy{
		RestrictedPaths: []string{"/etc"},
	}

	executor := NewSandboxedExecutor(policy)

	// Create a hook that violates security policy
	hook := Hook{
		Name:    "dangerous-hook",
		Command: "rm",
		Args:    []string{"-rf", "/"},
	}

	ctx := &HookContext{
		Event: EventPreCompletion,
	}

	result, err := executor.ExecuteHookSandboxed(context.Background(), hook, ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Hook should have failed security validation")
	}

	if !strings.Contains(result.Error, "security validation failed") {
		t.Errorf("Expected security validation error, got: %s", result.Error)
	}
}

func TestHookPermissionManager(t *testing.T) {
	tempDir := t.TempDir()
	permissionFile := filepath.Join(tempDir, "permissions.yaml")

	hpm := NewHookPermissionManager(permissionFile)

	// Initially, all hooks should be allowed (no restrictions)
	if !hpm.IsHookAllowed("test-hook") {
		t.Error("Hook should be allowed by default")
	}

	// Add a blocked hook
	hpm.AddBlockedHook("blocked-hook")
	if hpm.IsHookAllowed("blocked-hook") {
		t.Error("Blocked hook should not be allowed")
	}

	// Add an allowed hook
	hpm.AddAllowedHook("allowed-hook")
	hpm.AddAllowedHook("another-allowed-hook")

	// Now that we have an allow list, only allowed hooks should pass
	if !hpm.IsHookAllowed("allowed-hook") {
		t.Error("Explicitly allowed hook should be allowed")
	}

	if hpm.IsHookAllowed("random-hook") {
		t.Error("Hook not in allow list should be blocked")
	}

	// Blocked hooks should still be blocked even if in allow list
	hpm.AddAllowedHook("blocked-hook")
	if hpm.IsHookAllowed("blocked-hook") {
		t.Error("Hook should remain blocked even if added to allow list")
	}
}

func TestHookRateLimiter(t *testing.T) {
	// Create rate limiter with low limits for testing
	hrl := NewHookRateLimiter(2, 5) // 2 per minute, 5 per hour
	defer hrl.Stop()

	hookName := "test-hook"

	// Should allow first execution
	if !hrl.IsAllowed(hookName) {
		t.Error("First execution should be allowed")
	}
	hrl.RecordExecution(hookName)

	// Should allow second execution
	if !hrl.IsAllowed(hookName) {
		t.Error("Second execution should be allowed")
	}
	hrl.RecordExecution(hookName)

	// Should block third execution (exceeds per-minute limit)
	if hrl.IsAllowed(hookName) {
		t.Error("Third execution should be blocked")
	}

	// Different hook should be allowed
	if !hrl.IsAllowed("other-hook") {
		t.Error("Different hook should be allowed")
	}
}

func TestHookAuditLogger(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit.log")

	hal := NewHookAuditLogger(logFile, true)

	hook := Hook{
		Name:    "test-hook",
		Event:   EventPreCompletion,
		Command: "/bin/echo",
	}

	result := &HookResult{
		Success:  true,
		Output:   "Test output",
		Duration: time.Second,
		ExitCode: 0,
	}

	ctx := &HookContext{
		Event:      EventPreCompletion,
		WorkingDir: "/tmp",
	}

	err := hal.LogHookExecution(hook, result, ctx)
	if err != nil {
		t.Errorf("Unexpected error logging hook execution: %v", err)
	}

	// Test with disabled logger
	halDisabled := NewHookAuditLogger(logFile, false)
	err = halDisabled.LogHookExecution(hook, result, ctx)
	if err != nil {
		t.Errorf("Disabled logger should not error: %v", err)
	}
}

func TestValidateHookRegex(t *testing.T) {
	tests := []struct {
		name            string
		hookName        string
		allowedPatterns []string
		blockedPatterns []string
		expectError     bool
	}{
		{
			name:     "no patterns - allow all",
			hookName: "test-hook",
		},
		{
			name:            "allowed pattern match",
			hookName:        "test-hook",
			allowedPatterns: []string{"^test-.*"},
		},
		{
			name:            "allowed pattern no match",
			hookName:        "production-hook",
			allowedPatterns: []string{"^test-.*"},
			expectError:     true,
		},
		{
			name:            "blocked pattern match",
			hookName:        "dangerous-hook",
			blockedPatterns: []string{".*dangerous.*"},
			expectError:     true,
		},
		{
			name:            "blocked pattern no match",
			hookName:        "safe-hook",
			blockedPatterns: []string{".*dangerous.*"},
		},
		{
			name:            "allowed but also blocked",
			hookName:        "test-dangerous-hook",
			allowedPatterns: []string{"^test-.*"},
			blockedPatterns: []string{".*dangerous.*"},
			expectError:     true, // Blocked takes precedence
		},
		{
			name:            "invalid regex pattern",
			hookName:        "test-hook",
			allowedPatterns: []string{"[invalid"},
			expectError:     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateHookRegex(test.hookName, test.allowedPatterns, test.blockedPatterns)

			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestCreateRestrictedEnvironment(t *testing.T) {
	policy := &SecurityPolicy{}
	executor := NewSandboxedExecutor(policy)

	hook := Hook{
		Environment: map[string]string{
			"CUSTOM_VAR":  "value",
			"ANOTHER_VAR": "another_value",
			"PATH":        "/evil/path", // Should be ignored
		},
	}

	ctx := &HookContext{
		Event: EventPreCompletion,
	}

	env := executor.createRestrictedEnvironment(hook, ctx)

	// Check that safe environment variables are present
	hasPath := false
	hasHookIndicator := false
	hasEventIndicator := false
	hasCustomVar := false
	hasEvilPath := false

	for _, envVar := range env {
		if strings.HasPrefix(envVar, "PATH=") {
			hasPath = true
			if strings.Contains(envVar, "/evil/path") {
				hasEvilPath = true
			}
		}
		if envVar == "CGPT_HOOK=1" {
			hasHookIndicator = true
		}
		if envVar == "CGPT_HOOK_EVENT=pre-completion" {
			hasEventIndicator = true
		}
		if envVar == "CUSTOM_VAR=value" {
			hasCustomVar = true
		}
	}

	if !hasPath {
		t.Error("PATH should be set in restricted environment")
	}

	if hasEvilPath {
		t.Error("Hook should not be able to override safe PATH")
	}

	if !hasHookIndicator {
		t.Error("CGPT_HOOK indicator should be set")
	}

	if !hasEventIndicator {
		t.Error("CGPT_HOOK_EVENT should be set")
	}

	if !hasCustomVar {
		t.Error("Custom environment variable should be included")
	}
}

func BenchmarkSecurityValidation(b *testing.B) {
	policy := &SecurityPolicy{
		RestrictedPaths: []string{"/etc", "/sys", "/proc", "/dev"},
	}
	validator := NewSecurityValidator(policy)

	hook := Hook{
		Command: "/usr/bin/python3",
		Args:    []string{"script.py", "arg1", "arg2"},
		Environment: map[string]string{
			"VAR1": "value1",
			"VAR2": "value2",
		},
		WorkingDir: "/tmp",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateCommand(hook)
	}
}

func TestIsUnixSystem(t *testing.T) {
	// This test will vary by platform, just ensure it doesn't panic
	result := isUnixSystem()
	_ = result // Use the result to avoid unused variable warning
}

func TestContainsHelper(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !contains(slice, "apple") {
		t.Error("Should find 'apple' in slice")
	}

	if contains(slice, "orange") {
		t.Error("Should not find 'orange' in slice")
	}

	if contains([]string{}, "anything") {
		t.Error("Should not find anything in empty slice")
	}
}