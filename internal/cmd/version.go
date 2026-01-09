package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, build commit, and build date of drillmeasure`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("drillmeasure version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("date: %s\n", date)
	},
}

func newVersionCmd() *cobra.Command {
	return versionCmd
}

