package report

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/drillmeasure/drillmeasure/internal/config"
	"github.com/drillmeasure/drillmeasure/internal/runner"
)

// GenerateMarkdownReport creates a human-readable Markdown report
func GenerateMarkdownReport(result *runner.DrillResult) string {
	var b strings.Builder

	b.WriteString("# Drill Report\n\n")
	b.WriteString(fmt.Sprintf("**Scenario:** %s\n\n", result.Scenario.Name))
	if result.Scenario.Description != "" {
		b.WriteString(fmt.Sprintf("**Description:** %s\n\n", result.Scenario.Description))
	}
	b.WriteString(fmt.Sprintf("**Execution Time:** %s\n\n", result.StartTime.Format(time.RFC3339)))

	// Summary
	b.WriteString("## Summary\n\n")
	b.WriteString("| Metric | Target | Actual | Status |\n")
	b.WriteString("|--------|--------|--------|--------|\n")

	rtoStatus := "❌ FAIL"
	if result.RTOPassed {
		rtoStatus = "✅ PASS"
	}
	b.WriteString(fmt.Sprintf("| RTO | %s | %s | %s |\n",
		formatDuration(result.RTOTarget),
		formatDuration(result.RTOActual),
		rtoStatus))

	if result.RPOTarget > 0 {
		rpoStatus := "❌ FAIL"
		if result.RPOPassed {
			rpoStatus = "✅ PASS"
		}
		b.WriteString(fmt.Sprintf("| RPO | %s | N/A | %s |\n",
			formatDuration(result.RPOTarget),
			rpoStatus))
	}

	b.WriteString("\n")

	// Timeline
	b.WriteString("## Timeline\n\n")
	b.WriteString("| Event | Timestamp | Duration |\n")
	b.WriteString("|-------|-----------|----------|\n")
	b.WriteString(fmt.Sprintf("| Start | %s | - |\n", result.StartTime.Format(time.RFC3339)))

	if result.PreSnapshot != nil {
		b.WriteString(fmt.Sprintf("| Pre-snapshot | %s | %s |\n",
			result.PreSnapshot.Timestamp.Format(time.RFC3339),
			formatDuration(result.PreSnapshot.Duration)))
	}

	if result.Disrupt != nil {
		b.WriteString(fmt.Sprintf("| Disruption | %s | %s |\n",
			result.Disrupt.Timestamp.Format(time.RFC3339),
			formatDuration(result.Disrupt.Duration)))
	}

	if result.PostDisruptDelay > 0 {
		delayStart := result.Disrupt.Timestamp.Add(result.Disrupt.Duration)
		b.WriteString(fmt.Sprintf("| Post-disrupt delay | %s | %s |\n",
			delayStart.Format(time.RFC3339),
			formatDuration(result.PostDisruptDelay)))
	}

	if result.Recover != nil {
		b.WriteString(fmt.Sprintf("| Recovery | %s | %s |\n",
			result.Recover.Timestamp.Format(time.RFC3339),
			formatDuration(result.Recover.Duration)))
	}

	b.WriteString(fmt.Sprintf("| RTO measurement start | %s | - |\n",
		result.RTOStartTime.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("| RTO measurement end | %s | %s |\n",
		result.RTOEndTime.Format(time.RFC3339),
		formatDuration(result.RTOActual)))

	if result.PostSnapshot != nil {
		b.WriteString(fmt.Sprintf("| Post-snapshot | %s | %s |\n",
			result.PostSnapshot.Timestamp.Format(time.RFC3339),
			formatDuration(result.PostSnapshot.Duration)))
	}

	if result.RPOVerify != nil {
		b.WriteString(fmt.Sprintf("| RPO verification | %s | %s |\n",
			result.RPOVerify.Timestamp.Format(time.RFC3339),
			formatDuration(result.RPOVerify.Duration)))
	}

	b.WriteString(fmt.Sprintf("| End | %s | %s |\n",
		result.EndTime.Format(time.RFC3339),
		formatDuration(result.EndTime.Sub(result.StartTime))))

	b.WriteString("\n")

	// Health Check Attempts
	if len(result.HealthCheckAttempts) > 0 {
		b.WriteString("## Health Check Attempts\n\n")
		b.WriteString(fmt.Sprintf("Total attempts: %d\n\n", len(result.HealthCheckAttempts)))
		b.WriteString("| Attempt | Timestamp | Exit Code | Duration |\n")
		b.WriteString("|---------|-----------|-----------|----------|\n")
		for i, attempt := range result.HealthCheckAttempts {
			b.WriteString(fmt.Sprintf("| %d | %s | %d | %s |\n",
				i+1,
				attempt.Timestamp.Format(time.RFC3339),
				attempt.ExitCode,
				formatDuration(attempt.Duration)))
		}
		b.WriteString("\n")
	}

	// Command Details
	b.WriteString("## Command Execution Details\n\n")

	if result.PreSnapshot != nil {
		b.WriteString("### Pre-snapshot\n\n")
		b.WriteString(formatCommandResult(result.PreSnapshot))
	}

	if result.Disrupt != nil {
		b.WriteString("### Disruption\n\n")
		b.WriteString(formatCommandResult(result.Disrupt))
	}

	if result.Recover != nil {
		b.WriteString("### Recovery\n\n")
		b.WriteString(formatCommandResult(result.Recover))
	}

	if result.PostSnapshot != nil {
		b.WriteString("### Post-snapshot\n\n")
		b.WriteString(formatCommandResult(result.PostSnapshot))
	}

	if result.RPOVerify != nil {
		b.WriteString("### RPO Verification\n\n")
		b.WriteString(formatCommandResult(result.RPOVerify))
	}

	// Factor Logs
	if len(result.FactorLogs) > 0 {
		b.WriteString("## Influencing Factors\n\n")
		for i, log := range result.FactorLogs {
			b.WriteString(fmt.Sprintf("### Factor Log %d\n\n", i+1))
			b.WriteString(formatCommandResult(&log))
		}
	}

	// Errors
	if len(result.Errors) > 0 {
		b.WriteString("## Errors\n\n")
		for _, err := range result.Errors {
			b.WriteString(fmt.Sprintf("- %s\n", err))
		}
		b.WriteString("\n")
	}

	// Compliance Notes
	b.WriteString("## Compliance Notes\n\n")
	b.WriteString("This drill measures Recovery Time Objective (RTO) and Recovery Point Objective (RPO) ")
	b.WriteString("as part of disaster recovery and business continuity planning.\n\n")

	if result.RTOPassed {
		b.WriteString("- ✅ **RTO Compliance**: Service recovered within the target RTO.\n")
	} else {
		b.WriteString("- ❌ **RTO Compliance**: Service did not recover within the target RTO.\n")
	}

	if result.RPOTarget > 0 {
		if result.RPOPassed {
			b.WriteString("- ✅ **RPO Compliance**: Data loss verified to be within acceptable limits.\n")
		} else {
			b.WriteString("- ❌ **RPO Compliance**: Data loss verification failed or exceeded limits.\n")
		}
	}

	b.WriteString("\n")
	b.WriteString("This report can be used as evidence for:\n")
	b.WriteString("- SOC 2 Type II audits\n")
	b.WriteString("- ISO 27001 compliance\n")
	b.WriteString("- Internal disaster recovery planning\n")
	b.WriteString("- Service level agreement (SLA) validation\n\n")

	return b.String()
}

// formatCommandResult formats a command result for Markdown
func formatCommandResult(result *runner.CommandResult) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("**Command:** `%s`\n\n", result.Command))
	b.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", result.Timestamp.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**Duration:** %s\n\n", formatDuration(result.Duration)))
	b.WriteString(fmt.Sprintf("**Exit Code:** %d\n\n", result.ExitCode))

	if result.Stdout != "" {
		b.WriteString("**Stdout:**\n\n")
		b.WriteString("```\n")
		b.WriteString(result.Stdout)
		b.WriteString("\n```\n\n")
	}

	if result.Stderr != "" {
		b.WriteString("**Stderr:**\n\n")
		b.WriteString("```\n")
		b.WriteString(result.Stderr)
		b.WriteString("\n```\n\n")
	}

	b.WriteString(fmt.Sprintf("**Stdout Hash (SHA256):** `%s`\n\n", result.StdoutHash))
	b.WriteString(fmt.Sprintf("**Stderr Hash (SHA256):** `%s`\n\n", result.StderrHash))

	return b.String()
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return d.String()
}

// ReportData represents the JSON structure for reports
type ReportData struct {
	Scenario          *config.Scenario        `json:"scenario"`
	StartTime         string                  `json:"start_time"`
	EndTime           string                  `json:"end_time"`
	RTOTarget         string                  `json:"rto_target"`
	RTOActual         string                  `json:"rto_actual"`
	RTOPassed         bool                    `json:"rto_passed"`
	RPOTarget         string                  `json:"rpo_target,omitempty"`
	RPOPassed         bool                    `json:"rpo_passed,omitempty"`
	PreSnapshot       *CommandResultData      `json:"pre_snapshot,omitempty"`
	Disrupt           *CommandResultData      `json:"disrupt"`
	Recover           *CommandResultData      `json:"recover,omitempty"`
	PostDisruptDelay  string                  `json:"post_disrupt_delay,omitempty"`
	PostSnapshot      *CommandResultData      `json:"post_snapshot,omitempty"`
	RPOVerify         *CommandResultData      `json:"rpo_verify,omitempty"`
	HealthCheckAttempts []CommandResultData   `json:"health_check_attempts"`
	FactorLogs        []CommandResultData     `json:"factor_logs,omitempty"`
	Errors            []string                `json:"errors,omitempty"`
}

// CommandResultData represents command execution data in JSON
type CommandResultData struct {
	Command     string `json:"command"`
	ExitCode    int    `json:"exit_code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Duration    string `json:"duration"`
	Timestamp   string `json:"timestamp"`
	StdoutHash  string `json:"stdout_hash"`
	StderrHash  string `json:"stderr_hash"`
}

// GenerateJSONReport creates a machine-readable JSON report
func GenerateJSONReport(result *runner.DrillResult) (string, error) {
	data := ReportData{
		Scenario:          result.Scenario,
		StartTime:         result.StartTime.Format(time.RFC3339),
		EndTime:           result.EndTime.Format(time.RFC3339),
		RTOTarget:         formatDuration(result.RTOTarget),
		RTOActual:         formatDuration(result.RTOActual),
		RTOPassed:         result.RTOPassed,
		RPOPassed:         result.RPOPassed,
		PostDisruptDelay:  formatDuration(result.PostDisruptDelay),
		HealthCheckAttempts: make([]CommandResultData, 0, len(result.HealthCheckAttempts)),
		FactorLogs:        make([]CommandResultData, 0, len(result.FactorLogs)),
		Errors:            result.Errors,
	}

	if result.RPOTarget > 0 {
		data.RPOTarget = formatDuration(result.RPOTarget)
	}

	if result.PreSnapshot != nil {
		data.PreSnapshot = commandResultToData(result.PreSnapshot)
	}

	if result.Disrupt != nil {
		data.Disrupt = commandResultToData(result.Disrupt)
	}

	if result.Recover != nil {
		data.Recover = commandResultToData(result.Recover)
	}

	if result.PostSnapshot != nil {
		data.PostSnapshot = commandResultToData(result.PostSnapshot)
	}

	if result.RPOVerify != nil {
		data.RPOVerify = commandResultToData(result.RPOVerify)
	}

	for _, attempt := range result.HealthCheckAttempts {
		data.HealthCheckAttempts = append(data.HealthCheckAttempts, *commandResultToData(&attempt))
	}

	for _, log := range result.FactorLogs {
		data.FactorLogs = append(data.FactorLogs, *commandResultToData(&log))
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// commandResultToData converts a CommandResult to CommandResultData
func commandResultToData(result *runner.CommandResult) *CommandResultData {
	return &CommandResultData{
		Command:    result.Command,
		ExitCode:   result.ExitCode,
		Stdout:     result.Stdout,
		Stderr:     result.Stderr,
		Duration:   formatDuration(result.Duration),
		Timestamp:  result.Timestamp.Format(time.RFC3339),
		StdoutHash: result.StdoutHash,
		StderrHash: result.StderrHash,
	}
}

