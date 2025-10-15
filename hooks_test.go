package cgpt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHookManager_NewHookManager(t *testing.T) {
	config := &HookConfig{
		Enabled:         true,
		GlobalHooksDir:  "/tmp/cgpt-test/global-hooks",
		ProjectHooksDir: ".cgpt/hooks",
		Timeout:         10 * time.Second,
		EnableSandbox:   true,
	}

	hm := NewHookManager(config, nil)

	if hm == nil {
		t.Fatal("NewHookManager returned nil")
	}

	if hm.config != config {
		t.Error("Config not properly set")
	}

	if hm.sandboxedExecutor == nil {
		t.Error("Sandboxed executor should be initialized when EnableSandbox is true")
	}

	if hm.permissionManager == nil {
		t.Error("Permission manager should be initialized")
	}

	if hm.auditLogger == nil {
		t.Error("Audit logger should be initialized")
	}

	if hm.rateLimiter == nil {
		t.Error("Rate limiter should be initialized")
	}
}

func TestHookManager_DefaultConfig(t *testing.T) {
	hm := NewHookManager(nil, nil)

	if !hm.config.Enabled {
		t.Error("Default config should have hooks enabled")
	}

	if hm.config.Timeout == 0 {
		t.Error("Default config should have a timeout set")
	}

	if !hm.config.EnableSandbox {
		t.Error("Default config should enable sandbox")
	}
}

func TestHookValidation(t *testing.T) {
	hm := NewHookManager(nil, nil)

	validHook := Hook{
		Name:    "test-hook",
		Event:   EventPreCompletion,
		Command: "/bin/echo",
		Args:    []string{"test"},
	}

	if !hm.isValidHook(validHook) {
		t.Error("Valid hook should pass validation")
	}

	invalidHook1 := Hook{
		Name:    "", // Missing name
		Event:   EventPreCompletion,
		Command: "/bin/echo",
	}

	if hm.isValidHook(invalidHook1) {
		t.Error("Hook with missing name should fail validation")
	}

	invalidHook2 := Hook{
		Name:    "test-hook",
		Event:   EventPreCompletion,
		Command: "", // Missing command
	}

	if hm.isValidHook(invalidHook2) {
		t.Error("Hook with missing command should fail validation")
	}

	invalidHook3 := Hook{
		Name:    "test-hook",
		Event:   "invalid-event", // Invalid event
		Command: "/bin/echo",
	}

	if hm.isValidHook(invalidHook3) {
		t.Error("Hook with invalid event should fail validation")
	}
}

func TestHookConditionEvaluation(t *testing.T) {
	hm := NewHookManager(nil, nil)

	ctx := &HookContext{
		Event: EventPreCompletion,
		Config: &Config{
			Backend: "anthropic",
			Model:   "claude-sonnet-4-20250514",
		},
		Metadata: map[string]interface{}{
			"testField": "testValue",
		},
	}

	tests := []struct {
		name      string
		condition Condition
		expected  bool
	}{
		{
			name: "equals backend",
			condition: Condition{
				Field:    "backend",
				Operator: "equals",
				Value:    "anthropic",
			},
			expected: true,
		},
		{
			name: "not equals backend",
			condition: Condition{
				Field:    "backend",
				Operator: "not_equals",
				Value:    "openai",
			},
			expected: true,
		},
		{
			name: "contains model",
			condition: Condition{
				Field:    "model",
				Operator: "contains",
				Value:    "sonnet",
			},
			expected: true,
		},
		{
			name: "starts with model",
			condition: Condition{
				Field:    "model",
				Operator: "starts_with",
				Value:    "claude",
			},
			expected: true,
		},
		{
			name: "metadata field",
			condition: Condition{
				Field:    "testField",
				Operator: "equals",
				Value:    "testValue",
			},
			expected: true,
		},
		{
			name: "regex match",
			condition: Condition{
				Field:    "backend",
				Operator: "matches",
				Value:    "anth.*",
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := hm.evaluateCondition(test.condition, ctx)
			if result != test.expected {
				t.Errorf("Expected %v, got %v for condition: %+v", test.expected, result, test.condition)
			}
		})
	}
}

func TestHookSortByPriority(t *testing.T) {
	hooks := []Hook{
		{Name: "high", Priority: 1},
		{Name: "low", Priority: 10},
		{Name: "medium", Priority: 5},
		{Name: "highest", Priority: 0},
	}

	sortHooksByPriority(hooks)

	expectedOrder := []string{"highest", "high", "medium", "low"}
	for i, hook := range hooks {
		if hook.Name != expectedOrder[i] {
			t.Errorf("Expected hook %d to be %s, got %s", i, expectedOrder[i], hook.Name)
		}
	}
}

func TestHookContextCreation(t *testing.T) {
	config := &Config{
		Backend: "anthropic",
		Model:   "claude-sonnet-4-20250514",
	}

	ctx := CreateHookContext(EventPreCompletion, config)

	if ctx.Event != EventPreCompletion {
		t.Error("Event not properly set")
	}

	if ctx.Config != config {
		t.Error("Config not properly set")
	}

	if ctx.Timestamp.IsZero() {
		t.Error("Timestamp not set")
	}

	if ctx.Metadata == nil {
		t.Error("Metadata not initialized")
	}

	if ctx.Environment == nil {
		t.Error("Environment not initialized")
	}

	if ctx.WorkingDir == "" {
		t.Error("Working directory not set")
	}
}

func TestHookManagerExecuteHooks_DisabledHooks(t *testing.T) {
	config := &HookConfig{
		Enabled: false,
	}
	hm := NewHookManager(config, nil)

	ctx := CreateHookContext(EventPreCompletion, &Config{})
	results, err := hm.ExecuteHooks(context.Background(), EventPreCompletion, ctx)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if results != nil {
		t.Error("Results should be nil when hooks are disabled")
	}
}

// Integration test that creates temporary hooks and executes them
func TestHookExecution_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for hooks
	tempDir := t.TempDir()
	hooksDir := filepath.Join(tempDir, "hooks")
	err := os.MkdirAll(hooksDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create a simple test hook
	hookScript := `#!/bin/bash
echo "Test hook executed" >&2
exit 0
`
	hookPath := filepath.Join(hooksDir, "test-hook.sh")
	err = os.WriteFile(hookPath, []byte(hookScript), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create hook configuration
	config := &HookConfig{
		Enabled:        true,
		GlobalHooksDir: hooksDir,
		Timeout:        5 * time.Second,
		EnableSandbox:  false, // Disable sandbox for test
	}

	hm := NewHookManager(config, nil)

	// Add a test hook manually
	testHook := Hook{
		Name:      "test-hook",
		Event:     EventPreCompletion,
		Command:   hookPath,
		OnFailure: FailureActionWarn,
	}
	hm.globalHooks[EventPreCompletion] = []Hook{testHook}

	// Execute hooks
	ctx := CreateHookContext(EventPreCompletion, &Config{Backend: "test"})
	results, err := hm.ExecuteHooks(context.Background(), EventPreCompletion, ctx)

	if err != nil {
		t.Errorf("Hook execution failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if !results[0].Success {
		t.Errorf("Hook should have succeeded: %s", results[0].Error)
	}

	if !strings.Contains(results[0].Output, "Test hook executed") {
		t.Errorf("Hook output not as expected: %s", results[0].Output)
	}
}

func TestHookManagerLoadHooksFromDir(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()
	hooksDir := filepath.Join(tempDir, "hooks")
	preCompletionDir := filepath.Join(hooksDir, "pre-completion")

	err := os.MkdirAll(preCompletionDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create executable hook
	hookScript := `#!/bin/bash
echo "test"
`
	hookPath := filepath.Join(preCompletionDir, "test-hook.sh")
	err = os.WriteFile(hookPath, []byte(hookScript), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create hook manager and load hooks
	config := &HookConfig{
		Enabled:        true,
		GlobalHooksDir: hooksDir,
	}
	hm := NewHookManager(config, nil)

	// Check if hook was loaded
	if len(hm.globalHooks[EventPreCompletion]) == 0 {
		t.Error("Expected hook to be loaded from directory")
	}

	hook := hm.globalHooks[EventPreCompletion][0]
	if hook.Name != "test-hook.sh" {
		t.Errorf("Expected hook name to be 'test-hook.sh', got '%s'", hook.Name)
	}

	if hook.Event != EventPreCompletion {
		t.Errorf("Expected hook event to be pre-completion, got '%s'", hook.Event)
	}
}

func TestHookFailureActions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()

	// Create a failing hook
	failingScript := `#!/bin/bash
echo "Hook failed" >&2
exit 1
`
	hookPath := filepath.Join(tempDir, "failing-hook.sh")
	err := os.WriteFile(hookPath, []byte(failingScript), 0755)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		onFailure   FailureAction
		expectError bool
	}{
		{
			name:        "ignore failure",
			onFailure:   FailureActionIgnore,
			expectError: false,
		},
		{
			name:        "warn on failure",
			onFailure:   FailureActionWarn,
			expectError: false,
		},
		{
			name:        "abort on failure",
			onFailure:   FailureActionAbort,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &HookConfig{
				Enabled:       true,
				EnableSandbox: false,
			}
			hm := NewHookManager(config, nil)

			// Add failing hook
			failingHook := Hook{
				Name:      "failing-hook",
				Event:     EventPreCompletion,
				Command:   hookPath,
				OnFailure: test.onFailure,
			}
			hm.globalHooks[EventPreCompletion] = []Hook{failingHook}

			// Execute hooks
			ctx := CreateHookContext(EventPreCompletion, &Config{})
			_, err := hm.ExecuteHooks(context.Background(), EventPreCompletion, ctx)

			if test.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// Benchmark hook execution
func BenchmarkHookExecution(b *testing.B) {
	tempDir := b.TempDir()

	// Create a simple hook
	hookScript := `#!/bin/bash
exit 0
`
	hookPath := filepath.Join(tempDir, "bench-hook.sh")
	err := os.WriteFile(hookPath, []byte(hookScript), 0755)
	if err != nil {
		b.Fatal(err)
	}

	config := &HookConfig{
		Enabled:       true,
		EnableSandbox: false,
	}
	hm := NewHookManager(config, nil)

	hook := Hook{
		Name:      "bench-hook",
		Event:     EventPreCompletion,
		Command:   hookPath,
		OnFailure: FailureActionIgnore,
	}
	hm.globalHooks[EventPreCompletion] = []Hook{hook}

	ctx := CreateHookContext(EventPreCompletion, &Config{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hm.ExecuteHooks(context.Background(), EventPreCompletion, ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestCreateHookContext(t *testing.T) {
	config := &Config{
		Backend:     "anthropic",
		Model:       "claude-sonnet-4-20250514",
		MaxTokens:   4096,
		Temperature: 0.1,
	}

	event := EventPostCompletion
	ctx := CreateHookContext(event, config)

	// Verify basic fields
	if ctx.Event != event {
		t.Errorf("Expected event %s, got %s", event, ctx.Event)
	}

	if ctx.Config != config {
		t.Error("Config not properly assigned")
	}

	if ctx.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	// Verify metadata is initialized
	if ctx.Metadata == nil {
		t.Error("Metadata should be initialized")
	}

	// Verify environment is populated
	if ctx.Environment == nil {
		t.Error("Environment should be initialized")
	}

	// Should have at least some environment variables
	if len(ctx.Environment) == 0 {
		t.Error("Environment should contain some variables")
	}

	// Working directory should be set
	if ctx.WorkingDir == "" {
		t.Error("Working directory should be set")
	}
}

// Test hook filtering by conditions
func TestHookFiltering(t *testing.T) {
	hm := NewHookManager(nil, nil)

	hooks := []Hook{
		{
			Name:    "always-run",
			Event:   EventPreCompletion,
			Command: "/bin/true",
		},
		{
			Name:    "anthropic-only",
			Event:   EventPreCompletion,
			Command: "/bin/true",
			Conditions: []Condition{
				{
					Field:    "backend",
					Operator: "equals",
					Value:    "anthropic",
				},
			},
		},
		{
			Name:    "openai-only",
			Event:   EventPreCompletion,
			Command: "/bin/true",
			Conditions: []Condition{
				{
					Field:    "backend",
					Operator: "equals",
					Value:    "openai",
				},
			},
		},
	}

	ctx := &HookContext{
		Config: &Config{
			Backend: "anthropic",
		},
	}

	filtered := hm.filterHooksByConditions(hooks, ctx)

	// Should include always-run and anthropic-only, but not openai-only
	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered hooks, got %d", len(filtered))
	}

	names := make(map[string]bool)
	for _, hook := range filtered {
		names[hook.Name] = true
	}

	if !names["always-run"] {
		t.Error("Expected always-run hook to be included")
	}

	if !names["anthropic-only"] {
		t.Error("Expected anthropic-only hook to be included")
	}

	if names["openai-only"] {
		t.Error("Expected openai-only hook to be excluded")
	}
}
