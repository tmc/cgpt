package cgpt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// HookManager manages the lifecycle hooks for cgpt
type HookManager struct {
	config            *HookConfig
	globalHooks       map[HookEvent][]Hook
	projectHooks      map[HookEvent][]Hook
	logger            io.Writer
	sandboxedExecutor *SandboxedExecutor
	permissionManager *HookPermissionManager
	auditLogger       *HookAuditLogger
	rateLimiter       *HookRateLimiter
}

// HookConfig contains configuration for the hook system
type HookConfig struct {
	Enabled         bool              `yaml:"enabled" json:"enabled"`
	GlobalHooksDir  string            `yaml:"globalHooksDir" json:"globalHooksDir"`
	ProjectHooksDir string            `yaml:"projectHooksDir" json:"projectHooksDir"`
	Timeout         time.Duration     `yaml:"timeout" json:"timeout"`
	MaxConcurrency  int               `yaml:"maxConcurrency" json:"maxConcurrency"`
	SecurityPolicy  SecurityPolicy    `yaml:"securityPolicy" json:"securityPolicy"`
	AllowedCommands []string          `yaml:"allowedCommands" json:"allowedCommands"`
	BlockedCommands []string          `yaml:"blockedCommands" json:"blockedCommands"`
	Environment     map[string]string `yaml:"environment" json:"environment"`
	EnableSandbox   bool              `yaml:"enableSandbox" json:"enableSandbox"`
}

// SecurityPolicy defines security settings for hook execution
type SecurityPolicy struct {
	AllowNetworkAccess bool          `yaml:"allowNetworkAccess" json:"allowNetworkAccess"`
	AllowFileSystem    bool          `yaml:"allowFileSystem" json:"allowFileSystem"`
	RestrictedPaths    []string      `yaml:"restrictedPaths" json:"restrictedPaths"`
	MaxExecutionTime   time.Duration `yaml:"maxExecutionTime" json:"maxExecutionTime"`
	MaxMemoryUsage     int64         `yaml:"maxMemoryUsage" json:"maxMemoryUsage"` // in bytes
}

// HookEvent represents different lifecycle events
type HookEvent string

const (
	// Completion lifecycle events
	EventPreCompletion   HookEvent = "pre-completion"
	EventPostCompletion  HookEvent = "post-completion"
	EventCompletionError HookEvent = "completion-error"

	// History lifecycle events
	EventPreSave  HookEvent = "pre-save"
	EventPostSave HookEvent = "post-save"
	EventPreLoad  HookEvent = "pre-load"
	EventPostLoad HookEvent = "post-load"

	// Session lifecycle events
	EventSessionStart HookEvent = "session-start"
	EventSessionEnd   HookEvent = "session-end"

	// Interactive mode events
	EventInteractiveStart HookEvent = "interactive-start"
	EventInteractiveEnd   HookEvent = "interactive-end"
	EventUserInput        HookEvent = "user-input"

	// Configuration events
	EventConfigLoaded HookEvent = "config-loaded"

	// Custom events for extensibility
	EventCustom HookEvent = "custom"
)

// Hook represents a single hook
type Hook struct {
	Name        string            `yaml:"name" json:"name"`
	Event       HookEvent         `yaml:"event" json:"event"`
	Command     string            `yaml:"command" json:"command"`
	Args        []string          `yaml:"args" json:"args"`
	WorkingDir  string            `yaml:"workingDir" json:"workingDir"`
	Environment map[string]string `yaml:"environment" json:"environment"`
	Timeout     time.Duration     `yaml:"timeout" json:"timeout"`
	OnFailure   FailureAction     `yaml:"onFailure" json:"onFailure"`
	Conditions  []Condition       `yaml:"conditions" json:"conditions"`
	Priority    int               `yaml:"priority" json:"priority"` // Lower numbers execute first
}

// FailureAction defines what to do when a hook fails
type FailureAction string

const (
	FailureActionIgnore FailureAction = "ignore"
	FailureActionWarn   FailureAction = "warn"
	FailureActionAbort  FailureAction = "abort"
)

// Condition defines when a hook should execute
type Condition struct {
	Field    string `yaml:"field" json:"field"`       // e.g., "backend", "model", "continuous"
	Operator string `yaml:"operator" json:"operator"` // e.g., "equals", "contains", "matches"
	Value    string `yaml:"value" json:"value"`
}

// HookContext contains the execution context passed to hooks
type HookContext struct {
	Event       HookEvent              `json:"event"`
	Timestamp   time.Time              `json:"timestamp"`
	Config      *Config                `json:"config"`
	Messages    []llms.MessageContent  `json:"messages,omitempty"`
	Response    string                 `json:"response,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Environment map[string]string      `json:"environment,omitempty"`
	WorkingDir  string                 `json:"workingDir"`
	SessionID   string                 `json:"sessionId"`
}

// HookResult represents the result of hook execution
type HookResult struct {
	Success      bool                   `json:"success"`
	Output       string                 `json:"output"`
	Error        string                 `json:"error"`
	ExitCode     int                    `json:"exitCode"`
	Duration     time.Duration          `json:"duration"`
	ModifiedData map[string]interface{} `json:"modifiedData,omitempty"`
}

// NewHookManager creates a new hook manager
func NewHookManager(config *HookConfig, logger io.Writer) *HookManager {
	if config == nil {
		config = defaultHookConfig()
	}
	if logger == nil {
		logger = os.Stderr
	}

	hm := &HookManager{
		config:       config,
		globalHooks:  make(map[HookEvent][]Hook),
		projectHooks: make(map[HookEvent][]Hook),
		logger:       logger,
	}

	// Initialize security components
	if config.EnableSandbox {
		hm.sandboxedExecutor = NewSandboxedExecutor(&config.SecurityPolicy)
	}

	// Initialize permission manager
	permissionFile := filepath.Join(config.GlobalHooksDir, "permissions.yaml")
	hm.permissionManager = NewHookPermissionManager(permissionFile)
	hm.permissionManager.LoadPermissions()

	// Initialize audit logger
	auditFile := filepath.Join(config.GlobalHooksDir, "audit.log")
	hm.auditLogger = NewHookAuditLogger(auditFile, true)

	// Initialize rate limiter (5 per minute, 50 per hour by default)
	hm.rateLimiter = NewHookRateLimiter(5, 50)

	if config.Enabled {
		hm.loadHooks()
	}

	return hm
}

// defaultHookConfig returns the default hook configuration
func defaultHookConfig() *HookConfig {
	homeDir, _ := os.UserHomeDir()
	return &HookConfig{
		Enabled:         true,
		GlobalHooksDir:  filepath.Join(homeDir, ".cgpt", "hooks"),
		ProjectHooksDir: ".cgpt/hooks",
		Timeout:         30 * time.Second,
		MaxConcurrency:  5,
		EnableSandbox:   true,
		SecurityPolicy: SecurityPolicy{
			AllowNetworkAccess: false,
			AllowFileSystem:    true,
			MaxExecutionTime:   30 * time.Second,
			MaxMemoryUsage:     100 * 1024 * 1024, // 100MB
			RestrictedPaths:    []string{"/etc", "/sys", "/proc"},
		},
		Environment: map[string]string{
			"CGPT_HOOK": "1",
		},
	}
}

// loadHooks discovers and loads hooks from configured directories
func (hm *HookManager) loadHooks() {
	// Load global hooks
	if hm.config.GlobalHooksDir != "" {
		if err := hm.loadHooksFromDir(hm.config.GlobalHooksDir, hm.globalHooks); err != nil {
			fmt.Fprintf(hm.logger, "cgpt: warning: failed to load global hooks: %v\n", err)
		}
	}

	// Load project hooks
	if hm.config.ProjectHooksDir != "" {
		if err := hm.loadHooksFromDir(hm.config.ProjectHooksDir, hm.projectHooks); err != nil {
			fmt.Fprintf(hm.logger, "cgpt: warning: failed to load project hooks: %v\n", err)
		}
	}
}

// loadHooksFromDir loads hooks from a directory
func (hm *HookManager) loadHooksFromDir(dir string, hooksMap map[HookEvent][]Hook) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, skip
	}

	// Look for hook configuration files
	configFiles := []string{
		filepath.Join(dir, "hooks.yaml"),
		filepath.Join(dir, "hooks.yml"),
		filepath.Join(dir, "hooks.json"),
	}

	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			if err := hm.loadHooksFromFile(configFile, hooksMap); err != nil {
				return fmt.Errorf("failed to load hooks from %s: %w", configFile, err)
			}
			break
		}
	}

	// Also look for executable files in event-specific subdirectories
	return hm.loadExecutableHooks(dir, hooksMap)
}

// loadHooksFromFile loads hooks from a configuration file
func (hm *HookManager) loadHooksFromFile(filename string, hooksMap map[HookEvent][]Hook) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var hooks []Hook
	if strings.HasSuffix(filename, ".json") {
		if err := json.Unmarshal(data, &hooks); err != nil {
			return err
		}
	} else {
		// For now, implement basic YAML parsing for hooks
		// In a real implementation, you'd use a YAML library like gopkg.in/yaml.v3
		return fmt.Errorf("YAML support not implemented in this example")
	}

	for _, hook := range hooks {
		if !hm.isValidHook(hook) {
			fmt.Fprintf(hm.logger, "cgpt: warning: skipping invalid hook %s\n", hook.Name)
			continue
		}
		hooksMap[hook.Event] = append(hooksMap[hook.Event], hook)
	}

	return nil
}

// loadExecutableHooks loads executable hooks from event subdirectories
func (hm *HookManager) loadExecutableHooks(dir string, hooksMap map[HookEvent][]Hook) error {
	events := []HookEvent{
		EventPreCompletion, EventPostCompletion, EventCompletionError,
		EventPreSave, EventPostSave, EventPreLoad, EventPostLoad,
		EventSessionStart, EventSessionEnd,
		EventInteractiveStart, EventInteractiveEnd, EventUserInput,
		EventConfigLoaded, EventCustom,
	}

	for _, event := range events {
		eventDir := filepath.Join(dir, string(event))
		if _, err := os.Stat(eventDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(eventDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			path := filepath.Join(eventDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Check if file is executable
			if info.Mode()&0111 != 0 {
				hook := Hook{
					Name:      entry.Name(),
					Event:     event,
					Command:   path,
					OnFailure: FailureActionWarn,
					Timeout:   hm.config.Timeout,
				}
				hooksMap[event] = append(hooksMap[event], hook)
			}
		}
	}

	return nil
}

// isValidHook validates a hook configuration
func (hm *HookManager) isValidHook(hook Hook) bool {
	if hook.Name == "" || hook.Command == "" {
		return false
	}

	// Validate event
	validEvents := []HookEvent{
		EventPreCompletion, EventPostCompletion, EventCompletionError,
		EventPreSave, EventPostSave, EventPreLoad, EventPostLoad,
		EventSessionStart, EventSessionEnd,
		EventInteractiveStart, EventInteractiveEnd, EventUserInput,
		EventConfigLoaded, EventCustom,
	}

	for _, validEvent := range validEvents {
		if hook.Event == validEvent {
			return true
		}
	}

	return false
}

// ExecuteHooks executes all hooks for a given event
func (hm *HookManager) ExecuteHooks(ctx context.Context, event HookEvent, hookCtx *HookContext) ([]*HookResult, error) {
	if !hm.config.Enabled {
		return nil, nil
	}

	if hookCtx == nil {
		hookCtx = &HookContext{
			Event:     event,
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
	}

	hookCtx.Event = event
	hookCtx.Timestamp = time.Now()

	var allHooks []Hook

	// Collect hooks from both global and project scopes
	if globalHooks, exists := hm.globalHooks[event]; exists {
		allHooks = append(allHooks, globalHooks...)
	}
	if projectHooks, exists := hm.projectHooks[event]; exists {
		allHooks = append(allHooks, projectHooks...)
	}

	if len(allHooks) == 0 {
		return nil, nil
	}

	// Sort hooks by priority
	sortHooksByPriority(allHooks)

	// Filter hooks by conditions
	filteredHooks := hm.filterHooksByConditions(allHooks, hookCtx)

	var results []*HookResult

	for _, hook := range filteredHooks {
		// Check permissions
		if !hm.permissionManager.IsHookAllowed(hook.Name) {
			fmt.Fprintf(hm.logger, "cgpt: hook %s blocked by permissions\n", hook.Name)
			continue
		}

		// Check rate limits
		if !hm.rateLimiter.IsAllowed(hook.Name) {
			fmt.Fprintf(hm.logger, "cgpt: hook %s blocked by rate limiting\n", hook.Name)
			continue
		}

		// Record execution attempt
		hm.rateLimiter.RecordExecution(hook.Name)

		result, err := hm.executeHook(ctx, hook, hookCtx)
		if err != nil {
			result = &HookResult{
				Success:  false,
				Error:    err.Error(),
				Duration: 0,
			}
		}
		results = append(results, result)

		// Log execution for audit
		hm.auditLogger.LogHookExecution(hook, result, hookCtx)

		// Handle hook failure
		if !result.Success {
			switch hook.OnFailure {
			case FailureActionAbort:
				return results, fmt.Errorf("hook %s failed: %s", hook.Name, result.Error)
			case FailureActionWarn:
				fmt.Fprintf(hm.logger, "cgpt: warning: hook %s failed: %s\n", hook.Name, result.Error)
			case FailureActionIgnore:
				// Continue silently
			}
		}
	}

	return results, nil
}

// executeHook executes a single hook
func (hm *HookManager) executeHook(ctx context.Context, hook Hook, hookCtx *HookContext) (*HookResult, error) {
	// Use sandboxed execution if enabled
	if hm.config.EnableSandbox && hm.sandboxedExecutor != nil {
		return hm.sandboxedExecutor.ExecuteHookSandboxed(ctx, hook, hookCtx)
	}

	// Fallback to basic execution (with basic security validation)
	return hm.executeHookBasic(ctx, hook, hookCtx)
}

// executeHookBasic executes a hook with basic security (fallback method)
func (hm *HookManager) executeHookBasic(ctx context.Context, hook Hook, hookCtx *HookContext) (*HookResult, error) {
	start := time.Now()

	// Validate security policy
	if err := hm.validateSecurity(hook); err != nil {
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Set up timeout
	timeout := hook.Timeout
	if timeout == 0 {
		timeout = hm.config.Timeout
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Prepare command
	cmd := exec.CommandContext(execCtx, hook.Command, hook.Args...)

	// Set working directory
	if hook.WorkingDir != "" {
		cmd.Dir = hook.WorkingDir
	} else {
		if wd, err := os.Getwd(); err == nil {
			hookCtx.WorkingDir = wd
		}
	}

	// Set environment
	env := os.Environ()
	for k, v := range hm.config.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range hook.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Pass context as JSON via stdin
	contextJSON, err := json.Marshal(hookCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hook context: %w", err)
	}
	cmd.Stdin = strings.NewReader(string(contextJSON))

	// Capture output
	output, err := cmd.CombinedOutput()

	result := &HookResult{
		Success:  err == nil,
		Output:   string(output),
		Duration: time.Since(start),
	}

	if err != nil {
		result.Error = err.Error()
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				result.ExitCode = status.ExitStatus()
			}
		}
	}

	return result, nil
}

// filterHooksByConditions filters hooks based on their conditions
func (hm *HookManager) filterHooksByConditions(hooks []Hook, ctx *HookContext) []Hook {
	var filtered []Hook

	for _, hook := range hooks {
		if hm.evaluateConditions(hook.Conditions, ctx) {
			filtered = append(filtered, hook)
		}
	}

	return filtered
}

// evaluateConditions evaluates if all conditions are met
func (hm *HookManager) evaluateConditions(conditions []Condition, ctx *HookContext) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, condition := range conditions {
		if !hm.evaluateCondition(condition, ctx) {
			return false
		}
	}

	return true
}

// evaluateCondition evaluates a single condition
func (hm *HookManager) evaluateCondition(condition Condition, ctx *HookContext) bool {
	var fieldValue string

	// Get field value from context
	switch condition.Field {
	case "backend":
		if ctx.Config != nil {
			fieldValue = ctx.Config.Backend
		}
	case "model":
		if ctx.Config != nil {
			fieldValue = ctx.Config.Model
		}
	case "event":
		fieldValue = string(ctx.Event)
	default:
		// Check metadata
		if ctx.Metadata != nil {
			if value, exists := ctx.Metadata[condition.Field]; exists {
				fieldValue = fmt.Sprintf("%v", value)
			}
		}
	}

	// Evaluate operator
	switch condition.Operator {
	case "equals", "eq", "==":
		return fieldValue == condition.Value
	case "not_equals", "ne", "!=":
		return fieldValue != condition.Value
	case "contains":
		return strings.Contains(fieldValue, condition.Value)
	case "matches", "regex":
		matched, err := regexp.MatchString(condition.Value, fieldValue)
		return err == nil && matched
	case "starts_with":
		return strings.HasPrefix(fieldValue, condition.Value)
	case "ends_with":
		return strings.HasSuffix(fieldValue, condition.Value)
	default:
		return false
	}
}

// validateSecurity validates that the hook complies with security policies
func (hm *HookManager) validateSecurity(hook Hook) error {
	// Check blocked commands
	for _, blocked := range hm.config.BlockedCommands {
		if hook.Command == blocked || strings.Contains(hook.Command, blocked) {
			return fmt.Errorf("command %s is blocked", hook.Command)
		}
	}

	// Check allowed commands (if specified)
	if len(hm.config.AllowedCommands) > 0 {
		allowed := false
		for _, allowedCmd := range hm.config.AllowedCommands {
			if hook.Command == allowedCmd || strings.HasPrefix(hook.Command, allowedCmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command %s is not in allowed commands list", hook.Command)
		}
	}

	return nil
}

// sortHooksByPriority sorts hooks by priority (lower numbers first)
func sortHooksByPriority(hooks []Hook) {
	for i := 0; i < len(hooks)-1; i++ {
		for j := i + 1; j < len(hooks); j++ {
			if hooks[i].Priority > hooks[j].Priority {
				hooks[i], hooks[j] = hooks[j], hooks[i]
			}
		}
	}
}

// CreateHookContext creates a hook context from the current state
func CreateHookContext(event HookEvent, config *Config) *HookContext {
	ctx := &HookContext{
		Event:       event,
		Timestamp:   time.Now(),
		Config:      config,
		Metadata:    make(map[string]interface{}),
		Environment: make(map[string]string),
	}

	// Add environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			ctx.Environment[parts[0]] = parts[1]
		}
	}

	// Add working directory
	if wd, err := os.Getwd(); err == nil {
		ctx.WorkingDir = wd
	}

	return ctx
}
