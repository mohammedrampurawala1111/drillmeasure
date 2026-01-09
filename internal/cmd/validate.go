package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/drillmeasure/drillmeasure/internal/config"
)

var validateCmd = &cobra.Command{
	Use:   "validate <scenario.yaml>",
	Short: "Validate a scenario YAML file",
	Long: `Validate the syntax and required fields of a scenario YAML file.
This command checks:
- YAML syntax validity
- Presence of required fields
- Valid duration formats for RTO, RPO, and delays`,
	Args: cobra.ExactArgs(1),
	RunE: validateScenario,
}

func newValidateCmd() *cobra.Command {
	return validateCmd
}

func validateScenario(cmd *cobra.Command, args []string) error {
	scenarioPath := args[0]

	// Parse scenario
	scenario, err := config.ParseScenario(scenarioPath)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Validate fields
	if err := scenario.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Printf("✅ Scenario file is valid: %s\n", scenarioPath)
	fmt.Printf("   Name: %s\n", scenario.Name)
	fmt.Printf("   RTO Target: %s\n", scenario.RTOTarget)
	if scenario.RPOTarget != "" {
		fmt.Printf("   RPO Target: %s\n", scenario.RPOTarget)
	}

	return nil
}

