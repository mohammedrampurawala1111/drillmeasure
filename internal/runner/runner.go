package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/drillmeasure/drillmeasure/internal/config"
)

// CommandResult holds the result of executing a command
type CommandResult struct {
	Command     string
	ExitCode    int
	Stdout      string
	Stderr      string
	Duration    time.Duration
	Timestamp   time.Time
	StdoutHash  string
	StderrHash  string
}

// DrillResult holds the complete result of a drill execution
type DrillResult struct {
	Scenario          *config.Scenario
	StartTime         time.Time
	EndTime           time.Time
	PreSnapshot       *CommandResult
	Disrupt           *CommandResult
	Recover           *CommandResult
	PostDisruptDelay  time.Duration
	RTOStartTime      time.Time
	RTOEndTime        time.Time
	RTOActual         time.Duration
	RTOTarget         time.Duration
	RTOPassed         bool
	PostSnapshot      *CommandResult
	RPOVerify         *CommandResult
	RPOTarget         time.Duration
	RPOPassed         bool
	HealthCheckAttempts []CommandResult
	FactorLogs        []CommandResult
	Errors            []string
}

// Runner executes drill scenarios
type Runner struct {
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
}

// NewRunner creates a new runner with default settings
func NewRunner() *Runner {
	return &Runner{
		healthCheckInterval: 5 * time.Second,
		// Terraform apply or slow health checks may take longer; give a generous timeout
		healthCheckTimeout:  5 * time.Minute,
	}
}

// Run executes a complete drill scenario
func (r *Runner) Run(ctx context.Context, scenario *config.Scenario) (*DrillResult, error) {
	result := &DrillResult{
		Scenario: scenario,
		StartTime: time.Now(),
		Errors:    []string{},
	}

	// Parse durations
	rtoTarget, err := scenario.GetRTOTargetDuration()
	if err != nil {
		return nil, fmt.Errorf("invalid RTO target: %w", err)
	}
	result.RTOTarget = rtoTarget

	rpoTarget, err := scenario.GetRPOTargetDuration()
	if err != nil {
		return nil, fmt.Errorf("invalid RPO target: %w", err)
	}
	result.RPOTarget = rpoTarget

	postDisruptDelay, err := scenario.GetPostDisruptDelay()
	if err != nil {
		return nil, fmt.Errorf("invalid post_disrupt_delay: %w", err)
	}
	result.PostDisruptDelay = postDisruptDelay

	// Step 1: Pre-snapshot (if present)
	if scenario.RPOCheck != nil && scenario.RPOCheck.PreSnapshot != "" {
		result.PreSnapshot = r.executeCommand(ctx, scenario.RPOCheck.PreSnapshot)
		if result.PreSnapshot.ExitCode != 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("pre_snapshot command failed with exit code %d", result.PreSnapshot.ExitCode))
		}
	}

	// Step 2: Disrupt
	result.Disrupt = r.executeCommand(ctx, scenario.DisruptCommand)
	if result.Disrupt.ExitCode != 0 {
		result.Errors = append(result.Errors, fmt.Sprintf("disrupt_command failed with exit code %d", result.Disrupt.ExitCode))
	}

	// Step 3: Post-disrupt delay
	if postDisruptDelay > 0 {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(postDisruptDelay):
		}
	}

	// Step 4: Recover (if recover_command is present)
	if scenario.RecoverCommand != "" {
		fmt.Println("Executing recovery command...")
		result.Recover = r.executeCommand(ctx, scenario.RecoverCommand)
		if result.Recover.ExitCode != 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("recover_command failed with exit code %d", result.Recover.ExitCode))
		} else {
			fmt.Println("Recovery command completed successfully")
		}
	}

	// Step 5: RTO measurement - wait for health check to pass
	result.RTOStartTime = time.Now()
	result.RTOPassed = r.waitForHealthCheck(ctx, scenario.HealthCheckCommand, rtoTarget, result)

	// Step 6: Post-snapshot (if present)
	if scenario.RPOCheck != nil && scenario.RPOCheck.PostSnapshot != "" {
		result.PostSnapshot = r.executeCommand(ctx, scenario.RPOCheck.PostSnapshot)
		if result.PostSnapshot.ExitCode != 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("post_snapshot command failed with exit code %d", result.PostSnapshot.ExitCode))
		}
	}

	// Step 7: RPO verification (if present)
	if scenario.RPOCheck != nil && scenario.RPOCheck.VerifyCommand != "" {
		result.RPOVerify = r.executeCommand(ctx, scenario.RPOCheck.VerifyCommand)
		if result.RPOVerify.ExitCode == 0 {
			result.RPOPassed = true
		} else {
			result.RPOPassed = false
			result.Errors = append(result.Errors, fmt.Sprintf("rpo verify_command failed with exit code %d", result.RPOVerify.ExitCode))
		}
	} else if rpoTarget > 0 {
		// If RPO target is set but no verify command, we can't measure it
		result.Errors = append(result.Errors, "RPO target specified but no verify_command provided")
	}

	// Step 8: Collect factor logs
	if scenario.Factors != nil && len(scenario.Factors.LogCommands) > 0 {
		for _, logCmd := range scenario.Factors.LogCommands {
			logResult := r.executeCommand(ctx, logCmd)
			result.FactorLogs = append(result.FactorLogs, *logResult)
		}
	}

	result.EndTime = time.Now()
	result.RTOEndTime = result.EndTime

	return result, nil
}

// executeCommand runs a shell command and returns the result
func (r *Runner) executeCommand(ctx context.Context, command string) *CommandResult {
	result := &CommandResult{
		Command:   command,
		Timestamp: time.Now(),
	}

	start := time.Now()

	// Execute command via bash
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	
	// Capture both stdout and stderr separately for better debugging
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.StdoutHash = hashString(result.Stdout)
	result.StderrHash = hashString(result.Stderr)

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			if result.Stderr == "" {
				result.Stderr = fmt.Sprintf("command timed out: %v", err)
			}
		} else if ctx.Err() == context.Canceled {
			result.ExitCode = -1
			if result.Stderr == "" {
				result.Stderr = fmt.Sprintf("command canceled: %v", err)
			}
		} else {
			result.ExitCode = -1
			if result.Stderr == "" {
				result.Stderr = err.Error()
			}
		}
	} else {
		result.ExitCode = 0
	}

	result.Duration = time.Since(start)

	return result
}

// waitForHealthCheck repeatedly checks health until it passes or RTO target is exceeded
func (r *Runner) waitForHealthCheck(ctx context.Context, healthCheckCommand string, rtoTarget time.Duration, result *DrillResult) bool {
	deadline := result.RTOStartTime.Add(rtoTarget)
	attemptNum := 0

	for {
		// Check if we've exceeded RTO target
		if time.Now().After(deadline) {
			result.RTOActual = time.Since(result.RTOStartTime)
			return false
		}

		attemptNum++
		elapsed := time.Since(result.RTOStartTime)
		fmt.Printf("[Health Check #%d] Attempting health check (elapsed: %s)...\n", attemptNum, formatDuration(elapsed))
		
		// Create timeout context for this health check
		checkCtx, cancel := context.WithTimeout(ctx, r.healthCheckTimeout)
		attempt := r.executeCommand(checkCtx, healthCheckCommand)
		cancel()

		result.HealthCheckAttempts = append(result.HealthCheckAttempts, *attempt)

		if attempt.ExitCode == 0 {
			result.RTOActual = time.Since(result.RTOStartTime)
			result.RTOEndTime = time.Now()
			fmt.Printf("[Health Check #%d] ✅ Service is healthy!\n", attemptNum)
			return true
		}

		fmt.Printf("[Health Check #%d] ❌ Health check failed (exit code: %d). Retrying in %s...\n", 
			attemptNum, attempt.ExitCode, r.healthCheckInterval)
		if attempt.Stderr != "" {
			fmt.Printf("  Error: %s\n", strings.TrimSpace(attempt.Stderr))
		}

		// Wait before next attempt
		select {
		case <-ctx.Done():
			result.RTOActual = time.Since(result.RTOStartTime)
			return false
		case <-time.After(r.healthCheckInterval):
		}
	}
}

// hashString computes SHA256 hash of a string
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.String()
}

