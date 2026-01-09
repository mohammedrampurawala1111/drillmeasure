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
	RTOStartTime      time.Time  // When service actually went down (first failed health check)
	RTOEndTime        time.Time  // When service recovered (first successful health check)
	RTA               time.Duration  // Recovery Time Actual - measured downtime
	RTOTarget         time.Duration  // Recovery Time Objective - maximum acceptable downtime
	RTOPassed         bool  // Whether RTA <= RTOTarget
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

	// Step 4: Check health immediately after disruption to detect if service went down
	// This establishes when RTA starts (when service actually goes down)
	fmt.Println("Checking if disruption caused service downtime...")
	postDisruptCheck := r.executeCommand(ctx, scenario.HealthCheckCommand)
	result.HealthCheckAttempts = append(result.HealthCheckAttempts, *postDisruptCheck)
	
	if postDisruptCheck.ExitCode != 0 {
		// Service is down - RTA starts now
		result.RTOStartTime = postDisruptCheck.Timestamp
		fmt.Printf("Service is down - RTA measurement started at %s\n", result.RTOStartTime.Format(time.RFC3339))
	}

	// Step 5: Recover (if recover_command is present)
	if scenario.RecoverCommand != "" {
		fmt.Println("Executing recovery command...")
		result.Recover = r.executeCommand(ctx, scenario.RecoverCommand)
		if result.Recover.ExitCode != 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("recover_command failed with exit code %d", result.Recover.ExitCode))
		} else {
			fmt.Println("Recovery command completed successfully")
		}
	}

	// Step 6: RTA measurement - continue checking health until service recovers
	// If RTA already started (service was down), continue until it's healthy
	// If RTA hasn't started (service still healthy), wait for it to go down or stay healthy
	r.waitForHealthCheck(ctx, scenario.HealthCheckCommand, rtoTarget, result)

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
	// RTOEndTime is already set in waitForHealthCheck, but ensure it's set if we didn't run health checks
	if result.RTOEndTime.IsZero() {
		result.RTOEndTime = result.EndTime
		if !result.RTOStartTime.IsZero() {
			result.RTA = result.RTOEndTime.Sub(result.RTOStartTime)
			result.RTOPassed = result.RTA <= result.RTOTarget
		}
	}

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
// If RTA already started (RTOStartTime is set), continue checking until service recovers
// If RTA hasn't started, check if service goes down or stays healthy
func (r *Runner) waitForHealthCheck(ctx context.Context, healthCheckCommand string, rtoTarget time.Duration, result *DrillResult) bool {
	// Check if RTA already started (service was detected as down after disruption)
	rtaStarted := !result.RTOStartTime.IsZero()
	attemptNum := len(result.HealthCheckAttempts)  // Continue from existing attempts

	for {
		attemptNum++
		
		// Create timeout context for this health check
		checkCtx, cancel := context.WithTimeout(ctx, r.healthCheckTimeout)
		attempt := r.executeCommand(checkCtx, healthCheckCommand)
		cancel()

		result.HealthCheckAttempts = append(result.HealthCheckAttempts, *attempt)

		if attempt.ExitCode == 0 {
			// Service is healthy
			if rtaStarted {
				// RTA ends when service becomes healthy again (first successful health check)
				result.RTOEndTime = time.Now()
				result.RTA = result.RTOEndTime.Sub(result.RTOStartTime)
				// Compare RTA vs RTO target
				result.RTOPassed = result.RTA <= rtoTarget
				fmt.Printf("[Health Check #%d] ✅ Service is healthy! RTA: %s (target RTO: %s) - %s\n", 
					attemptNum, formatDuration(result.RTA), formatDuration(rtoTarget), 
					map[bool]string{true: "✅ PASS", false: "❌ FAIL"}[result.RTOPassed])
				return result.RTOPassed
			} else {
				// Service never went down - disruption didn't cause downtime
				result.RTOPassed = true  // No downtime means we passed
				fmt.Printf("[Health Check #%d] ✅ Service is healthy (disruption did not cause downtime)\n", attemptNum)
				return true
			}
		} else {
			// Service is down
			if !rtaStarted {
				// RTA starts when service first goes down (shouldn't happen here if we checked after disruption)
				result.RTOStartTime = attempt.Timestamp
				rtaStarted = true
				fmt.Printf("[Health Check #%d] ❌ Service is down - RTA measurement started\n", attemptNum)
			}

			// Check if we've exceeded RTO target (from when service went down)
			now := time.Now()
			deadline := result.RTOStartTime.Add(rtoTarget)
			if now.After(deadline) {
				result.RTA = now.Sub(result.RTOStartTime)
				result.RTOEndTime = now
				result.RTOPassed = false  // RTA exceeded RTO target
				fmt.Printf("[Health Check #%d] ❌ RTO target exceeded! RTA: %s (target RTO: %s) - ❌ FAIL\n", 
					attemptNum, formatDuration(result.RTA), formatDuration(rtoTarget))
				return false
			}

			elapsed := now.Sub(result.RTOStartTime)
			remaining := deadline.Sub(now)
			fmt.Printf("[Health Check #%d] ❌ Health check failed (exit code: %d). RTA elapsed: %s, RTO remaining: %s. Retrying in %s...\n", 
				attemptNum, attempt.ExitCode, formatDuration(elapsed), formatDuration(remaining), r.healthCheckInterval)
			if attempt.Stderr != "" {
				fmt.Printf("  Error: %s\n", strings.TrimSpace(attempt.Stderr))
			}

			// Wait before next attempt
			select {
			case <-ctx.Done():
				if rtaStarted {
					result.RTA = time.Since(result.RTOStartTime)
					result.RTOEndTime = time.Now()
					result.RTOPassed = result.RTA <= rtoTarget
				}
				return false
			case <-time.After(r.healthCheckInterval):
			}
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

