package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/drillmeasure/drillmeasure/internal/config"
	"github.com/drillmeasure/drillmeasure/internal/report"
	"github.com/drillmeasure/drillmeasure/internal/runner"
)

var runCmd = &cobra.Command{
	Use:   "run <scenario.yaml>",
	Short: "Execute a drill scenario",
	Long: `Execute a complete drill scenario defined in a YAML file.
The tool will:
1. Execute pre-snapshot commands (if present)
2. Run the disruption command
3. Wait for the post-disrupt delay (if set)
4. Measure RTO by repeatedly checking health until service recovers
5. Execute post-snapshot commands (if present)
6. Verify RPO (if configured)
7. Collect factor logs
8. Generate Markdown and JSON reports`,
	Args: cobra.ExactArgs(1),
	RunE: runScenario,
}

func newRunCmd() *cobra.Command {
	return runCmd
}

func runScenario(cmd *cobra.Command, args []string) error {
	scenarioPath := args[0]

	// Parse scenario
	scenario, err := config.ParseScenario(scenarioPath)
	if err != nil {
		return fmt.Errorf("failed to parse scenario: %w", err)
	}

	// Validate scenario
	if err := scenario.Validate(); err != nil {
		return fmt.Errorf("scenario validation failed: %w", err)
	}

	fmt.Printf("Running scenario: %s\n", scenario.Name)
	if scenario.Description != "" {
		fmt.Printf("Description: %s\n", scenario.Description)
	}
	fmt.Println()

	// Create output directory
	outputDir, err := createOutputDirectory(scenario.Name)
	if err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("Output directory: %s\n\n", outputDir)

	// Create runner and execute
	r := runner.NewRunner()
	ctx := context.Background()

	fmt.Println("Starting drill execution...")
	fmt.Println("(This may take a while - health checks run every 5 seconds until service recovers)")
	fmt.Println("Note: Terraform operations may take 2-5 minutes. Please be patient...")
	result, err := r.Run(ctx, scenario)
	if err != nil {
		return fmt.Errorf("drill execution failed: %w", err)
	}

	// Generate reports
	if err := generateReports(result, outputDir); err != nil {
		return fmt.Errorf("failed to generate reports: %w", err)
	}

	// Print summary
	fmt.Println("Drill completed!")
	if result.RTOStartTime.IsZero() {
		// Service never went down
		fmt.Printf("Result: Disruption did not cause downtime - ✅ PASS (service remained healthy)\n")
	} else {
		fmt.Printf("RTA: %s (RTO target: %s) - ", formatDuration(result.RTA), formatDuration(result.RTOTarget))
		if result.RTOPassed {
			fmt.Println("✅ PASS")
		} else {
			fmt.Println("❌ FAIL")
		}
	}

	if result.RPOTarget > 0 {
		fmt.Printf("RPO: ")
		if result.RPOPassed {
			fmt.Println("✅ PASS")
		} else {
			fmt.Println("❌ FAIL")
		}
	}

	fmt.Printf("\nReports generated in: %s\n", outputDir)

	return nil
}

// createOutputDirectory creates a timestamped output directory
func createOutputDirectory(scenarioName string) (string, error) {
	timestamp := time.Now().Format("2006-01-02-150405")
	safeName := sanitizeFileName(scenarioName)
	dirName := fmt.Sprintf("%s-%s", timestamp, safeName)
	outputDir := filepath.Join("reports", dirName)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	return outputDir, nil
}

// sanitizeFileName removes unsafe characters from a filename
func sanitizeFileName(name string) string {
	var result []rune
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result = append(result, r)
		} else if r == ' ' {
			result = append(result, '-')
		}
	}
	return string(result)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return d.String()
}

// generateReports creates both Markdown and JSON reports
func generateReports(result *runner.DrillResult, outputDir string) error {
	// Generate Markdown report
	mdReport := report.GenerateMarkdownReport(result)
	mdPath := fmt.Sprintf("%s/report.md", outputDir)
	if err := os.WriteFile(mdPath, []byte(mdReport), 0644); err != nil {
		return fmt.Errorf("failed to write markdown report: %w", err)
	}

	// Generate JSON report
	jsonReport, err := report.GenerateJSONReport(result)
	if err != nil {
		return fmt.Errorf("failed to generate JSON report: %w", err)
	}
	jsonPath := fmt.Sprintf("%s/report.json", outputDir)
	if err := os.WriteFile(jsonPath, []byte(jsonReport), 0644); err != nil {
		return fmt.Errorf("failed to write JSON report: %w", err)
	}

	return nil
}

