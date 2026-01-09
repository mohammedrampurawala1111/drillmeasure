package cmd

import (
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
	commit  = "dev"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "drillmeasure",
	Short: "Measure RTO and RPO in any infrastructure environment",
	Long: `drillmeasure is a platform-agnostic CLI tool for measuring
Recovery Time Objective (RTO) and Recovery Point Objective (RPO)
in any infrastructure environment.

It executes user-provided commands for disruption, health checks,
snapshots, and log collection, making it compatible with Kubernetes,
VMs, cloud services, on-premises infrastructure, and hybrid setups.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newVersionCmd())
}

