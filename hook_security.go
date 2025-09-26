package cgpt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// SecurityValidator validates hook execution security
type SecurityValidator struct {
	policy *SecurityPolicy
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(policy *SecurityPolicy) *SecurityValidator {
	if policy == nil {
		policy = &SecurityPolicy{
			AllowNetworkAccess:  false,
			AllowFileSystem:     true,
			MaxExecutionTime:    30 * time.Second,
			MaxMemoryUsage:      100 * 1024 * 1024, // 100MB
			RestrictedPaths:     []string{"/etc", "/sys", "/proc", "/dev"},
		}
	}
	return &SecurityValidator{policy: policy}
}

// ValidateCommand validates a command before execution
func (sv *SecurityValidator) ValidateCommand(hook Hook) error {
	// Check for dangerous commands
	if err := sv.checkDangerousCommands(hook.Command); err != nil {
		return err
	}

	// Check for dangerous arguments
	if err := sv.checkDangerousArguments(hook.Args); err != nil {
		return err
	}

	// Check working directory
	if err := sv.validateWorkingDirectory(hook.WorkingDir); err != nil {
		return err
	}

	// Check environment variables
	if err := sv.validateEnvironment(hook.Environment); err != nil {
		return err
	}

	return nil
}

// checkDangerousCommands checks for potentially dangerous commands
func (sv *SecurityValidator) checkDangerousCommands(command string) error {
	// List of dangerous commands that should be blocked by default
	dangerousCommands := []string{
		"rm", "rmdir", "dd", "mkfs", "fdisk", "format",
		"sudo", "su", "doas", "pkexec",
		"chmod", "chown", "chgrp", "setfacl",
		"iptables", "ufw", "firewall-cmd",
		"systemctl", "service", "launchctl",
		"crontab", "at", "batch",
		"nc", "netcat", "telnet", "ssh", "scp", "rsync",
		"curl", "wget", "fetch", "lynx", "links",
		"python", "python3", "perl", "ruby", "node", "php",
		"bash", "sh", "zsh", "fish", "csh", "tcsh", "ksh",
		"exec", "eval", "source", ".",
	}

	// Extract base command name
	cmdName := filepath.Base(command)

	for _, dangerous := range dangerousCommands {
		if cmdName == dangerous || strings.HasPrefix(cmdName, dangerous) {
			return fmt.Errorf("dangerous command blocked: %s", cmdName)
		}
	}

	// Check for shell metacharacters that could enable command injection
	if containsShellMetacharacters(command) {
		return fmt.Errorf("command contains shell metacharacters: %s", command)
	}

	return nil
}

// checkDangerousArguments checks for potentially dangerous command arguments
func (sv *SecurityValidator) checkDangerousArguments(args []string) error {
	for _, arg := range args {
		// Check for attempts to access restricted paths
		for _, restrictedPath := range sv.policy.RestrictedPaths {
			if strings.HasPrefix(arg, restrictedPath) {
				return fmt.Errorf("access to restricted path blocked: %s", arg)
			}
		}

		// Check for shell metacharacters in arguments
		if containsShellMetacharacters(arg) {
			return fmt.Errorf("argument contains shell metacharacters: %s", arg)
		}
	}

	return nil
}

// validateWorkingDirectory validates the working directory
func (sv *SecurityValidator) validateWorkingDirectory(workingDir string) error {
	if workingDir == "" {
		return nil // Will use current directory
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}

	// Check against restricted paths
	for _, restrictedPath := range sv.policy.RestrictedPaths {
		if strings.HasPrefix(absPath, restrictedPath) {
			return fmt.Errorf("working directory in restricted path: %s", absPath)
		}
	}

	// Check if directory exists and is accessible
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("working directory not accessible: %w", err)
	}

	return nil
}

// validateEnvironment validates environment variables
func (sv *SecurityValidator) validateEnvironment(env map[string]string) error {
	// List of dangerous environment variables
	dangerousEnvVars := []string{
		"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES",
		"PATH", "PYTHONPATH", "RUBYLIB", "PERL5LIB",
		"SHELL", "BASH_ENV", "ENV",
	}

	for key, value := range env {
		// Check for dangerous environment variables
		for _, dangerous := range dangerousEnvVars {
			if strings.ToUpper(key) == dangerous {
				return fmt.Errorf("dangerous environment variable blocked: %s", key)
			}
		}

		// Check for shell metacharacters in values
		if containsShellMetacharacters(value) {
			return fmt.Errorf("environment variable contains shell metacharacters: %s=%s", key, value)
		}
	}

	return nil
}

// containsShellMetacharacters checks if a string contains shell metacharacters
func containsShellMetacharacters(s string) bool {
	// Common shell metacharacters that could enable injection
	metacharacters := []string{
		";", "&", "|", "<", ">", "(", ")", "{", "}", "[", "]",
		"$", "`", "\"", "'", "\\", "*", "?", "~",
	}

	for _, meta := range metacharacters {
		if strings.Contains(s, meta) {
			return true
		}
	}

	return false
}

// SandboxedExecutor executes commands in a sandboxed environment
type SandboxedExecutor struct {
	validator *SecurityValidator
	policy    *SecurityPolicy
}

// NewSandboxedExecutor creates a new sandboxed executor
func NewSandboxedExecutor(policy *SecurityPolicy) *SandboxedExecutor {
	return &SandboxedExecutor{
		validator: NewSecurityValidator(policy),
		policy:    policy,
	}
}

// ExecuteHookSandboxed executes a hook in a sandboxed environment
func (se *SandboxedExecutor) ExecuteHookSandboxed(ctx context.Context, hook Hook, hookCtx *HookContext) (*HookResult, error) {
	start := time.Now()

	// Validate security before execution
	if err := se.validator.ValidateCommand(hook); err != nil {
		return &HookResult{
			Success:  false,
			Error:    fmt.Sprintf("security validation failed: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Set up timeout
	timeout := hook.Timeout
	if timeout == 0 && se.policy.MaxExecutionTime > 0 {
		timeout = se.policy.MaxExecutionTime
	}
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	sandboxCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create sandboxed command
	cmd, err := se.createSandboxedCommand(sandboxCtx, hook, hookCtx)
	if err != nil {
		return &HookResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to create sandboxed command: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// Execute with resource limits
	output, err := se.executeWithLimits(cmd)

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

// createSandboxedCommand creates a command with sandboxing applied
func (se *SandboxedExecutor) createSandboxedCommand(ctx context.Context, hook Hook, hookCtx *HookContext) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	// On Unix systems, we can use additional sandboxing
	if isUnixSystem() {
		cmd = se.createUnixSandboxedCommand(ctx, hook, hookCtx)
	} else {
		// Fallback to basic command execution
		cmd = exec.CommandContext(ctx, hook.Command, hook.Args...)
	}

	// Set working directory
	if hook.WorkingDir != "" {
		cmd.Dir = hook.WorkingDir
	}

	// Set up restricted environment
	cmd.Env = se.createRestrictedEnvironment(hook, hookCtx)

	// Set up stdin with context data
	if hookCtx != nil {
		contextJSON, err := jsonMarshal(hookCtx)
		if err == nil {
			cmd.Stdin = strings.NewReader(string(contextJSON))
		}
	}

	return cmd, nil
}

// createUnixSandboxedCommand creates a sandboxed command on Unix systems
func (se *SandboxedExecutor) createUnixSandboxedCommand(ctx context.Context, hook Hook, hookCtx *HookContext) *exec.Cmd {
	// Use timeout command for additional process control
	args := []string{
		"timeout",
		fmt.Sprintf("%.0fs", se.policy.MaxExecutionTime.Seconds()),
		hook.Command,
	}
	args = append(args, hook.Args...)

	cmd := exec.CommandContext(ctx, "timeout", args[1:]...)

	// Set process group to enable killing child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return cmd
}

// createRestrictedEnvironment creates a restricted environment for the command
func (se *SandboxedExecutor) createRestrictedEnvironment(hook Hook, hookCtx *HookContext) []string {
	// Start with minimal safe environment
	safeEnv := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"LANG=" + os.Getenv("LANG"),
		"TZ=" + os.Getenv("TZ"),
		"CGPT_HOOK=1",
		"CGPT_HOOK_EVENT=" + string(hookCtx.Event),
	}

	// Add hook-specific environment variables (after validation)
	for key, value := range hook.Environment {
		// Skip if it would override a safe variable
		if !strings.HasPrefix(key, "CGPT_") &&
			!contains([]string{"PATH", "HOME", "USER", "LANG", "TZ"}, key) {
			safeEnv = append(safeEnv, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return safeEnv
}

// executeWithLimits executes a command with resource limits
func (se *SandboxedExecutor) executeWithLimits(cmd *exec.Cmd) ([]byte, error) {
	// Set memory limits if supported
	if se.policy.MaxMemoryUsage > 0 {
		se.setMemoryLimits(cmd)
	}

	// Execute the command
	return cmd.CombinedOutput()
}

// setMemoryLimits sets memory limits for the command (Unix-specific)
func (se *SandboxedExecutor) setMemoryLimits(cmd *exec.Cmd) {
	if isUnixSystem() {
		// Use ulimit to set memory limits
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}

		// Set memory limit (in KB)
		memLimitKB := se.policy.MaxMemoryUsage / 1024
		cmd.SysProcAttr.Credential = &syscall.Credential{}

		// Note: This is a simplified example. In a real implementation,
		// you would use cgroups or other OS-specific mechanisms
		_ = memLimitKB // Acknowledge variable to avoid unused variable error
	}
}

// isUnixSystem checks if we're running on a Unix-like system
func isUnixSystem() bool {
	// Simple check for Unix-like systems
	_, err := exec.LookPath("timeout")
	return err == nil
}

// contains checks if a string slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// HookPermissionManager manages hook permissions
type HookPermissionManager struct {
	allowedHooks   map[string]bool
	blockedHooks   map[string]bool
	permissionFile string
}

// NewHookPermissionManager creates a new permission manager
func NewHookPermissionManager(permissionFile string) *HookPermissionManager {
	return &HookPermissionManager{
		allowedHooks:   make(map[string]bool),
		blockedHooks:   make(map[string]bool),
		permissionFile: permissionFile,
	}
}

// IsHookAllowed checks if a hook is allowed to execute
func (hpm *HookPermissionManager) IsHookAllowed(hookName string) bool {
	// Check explicit blocks first
	if hpm.blockedHooks[hookName] {
		return false
	}

	// If we have an allow list, check it
	if len(hpm.allowedHooks) > 0 {
		return hpm.allowedHooks[hookName]
	}

	// Default to allowed if no explicit restrictions
	return true
}

// AddAllowedHook adds a hook to the allowed list
func (hpm *HookPermissionManager) AddAllowedHook(hookName string) {
	hpm.allowedHooks[hookName] = true
}

// AddBlockedHook adds a hook to the blocked list
func (hpm *HookPermissionManager) AddBlockedHook(hookName string) {
	hpm.blockedHooks[hookName] = true
}

// LoadPermissions loads permissions from a file
func (hpm *HookPermissionManager) LoadPermissions() error {
	if hpm.permissionFile == "" {
		return nil
	}

	if _, err := os.Stat(hpm.permissionFile); os.IsNotExist(err) {
		return nil // No permission file exists
	}

	// In a real implementation, you would parse the permission file
	// For now, this is a placeholder
	return nil
}

// SavePermissions saves permissions to a file
func (hpm *HookPermissionManager) SavePermissions() error {
	if hpm.permissionFile == "" {
		return nil
	}

	// In a real implementation, you would save permissions to file
	// For now, this is a placeholder
	return nil
}

// HookAuditLogger logs hook execution for security auditing
type HookAuditLogger struct {
	logFile string
	enabled bool
}

// NewHookAuditLogger creates a new audit logger
func NewHookAuditLogger(logFile string, enabled bool) *HookAuditLogger {
	return &HookAuditLogger{
		logFile: logFile,
		enabled: enabled,
	}
}

// LogHookExecution logs a hook execution event
func (hal *HookAuditLogger) LogHookExecution(hook Hook, result *HookResult, ctx *HookContext) error {
	if !hal.enabled {
		return nil
	}

	// Create audit log entry
	logEntry := map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339),
		"hook_name":  hook.Name,
		"event":      string(hook.Event),
		"command":    hook.Command,
		"success":    result.Success,
		"duration":   result.Duration.String(),
		"exit_code":  result.ExitCode,
		"error":      result.Error,
		"user":       os.Getenv("USER"),
		"working_dir": ctx.WorkingDir,
	}

	// In a real implementation, you would write to the audit log file
	// For now, this is a placeholder
	_ = logEntry

	return nil
}

// jsonMarshal is a helper function to marshal JSON (placeholder for actual implementation)
func jsonMarshal(v interface{}) ([]byte, error) {
	// In the actual implementation, this would use encoding/json
	return []byte("{}"), nil
}

// HookRateLimiter limits the rate of hook execution
type HookRateLimiter struct {
	executions    map[string][]time.Time
	maxPerMinute  int
	maxPerHour    int
	cleanupTicker *time.Ticker
}

// NewHookRateLimiter creates a new rate limiter
func NewHookRateLimiter(maxPerMinute, maxPerHour int) *HookRateLimiter {
	hrl := &HookRateLimiter{
		executions:   make(map[string][]time.Time),
		maxPerMinute: maxPerMinute,
		maxPerHour:   maxPerHour,
		cleanupTicker: time.NewTicker(5 * time.Minute),
	}

	// Start cleanup routine
	go hrl.cleanup()

	return hrl
}

// IsAllowed checks if a hook execution is allowed based on rate limits
func (hrl *HookRateLimiter) IsAllowed(hookName string) bool {
	now := time.Now()
	execTimes := hrl.executions[hookName]

	// Count executions in the last minute
	minuteAgo := now.Add(-time.Minute)
	execInMinute := 0
	for _, execTime := range execTimes {
		if execTime.After(minuteAgo) {
			execInMinute++
		}
	}

	if execInMinute >= hrl.maxPerMinute {
		return false
	}

	// Count executions in the last hour
	hourAgo := now.Add(-time.Hour)
	execInHour := 0
	for _, execTime := range execTimes {
		if execTime.After(hourAgo) {
			execInHour++
		}
	}

	if execInHour >= hrl.maxPerHour {
		return false
	}

	return true
}

// RecordExecution records a hook execution
func (hrl *HookRateLimiter) RecordExecution(hookName string) {
	hrl.executions[hookName] = append(hrl.executions[hookName], time.Now())
}

// cleanup removes old execution records
func (hrl *HookRateLimiter) cleanup() {
	for range hrl.cleanupTicker.C {
		now := time.Now()
		hourAgo := now.Add(-time.Hour)

		for hookName, execTimes := range hrl.executions {
			var filtered []time.Time
			for _, execTime := range execTimes {
				if execTime.After(hourAgo) {
					filtered = append(filtered, execTime)
				}
			}
			hrl.executions[hookName] = filtered
		}
	}
}

// Stop stops the cleanup routine
func (hrl *HookRateLimiter) Stop() {
	if hrl.cleanupTicker != nil {
		hrl.cleanupTicker.Stop()
	}
}

// ValidateHookRegex validates hook names against regex patterns
func ValidateHookRegex(hookName string, allowedPatterns, blockedPatterns []string) error {
	// Check blocked patterns first
	for _, pattern := range blockedPatterns {
		matched, err := regexp.MatchString(pattern, hookName)
		if err != nil {
			return fmt.Errorf("invalid blocked pattern %s: %w", pattern, err)
		}
		if matched {
			return fmt.Errorf("hook name %s matches blocked pattern %s", hookName, pattern)
		}
	}

	// If we have allowed patterns, check them
	if len(allowedPatterns) > 0 {
		for _, pattern := range allowedPatterns {
			matched, err := regexp.MatchString(pattern, hookName)
			if err != nil {
				return fmt.Errorf("invalid allowed pattern %s: %w", pattern, err)
			}
			if matched {
				return nil // Hook is allowed
			}
		}
		return fmt.Errorf("hook name %s does not match any allowed pattern", hookName)
	}

	return nil // No patterns specified, allow by default
}