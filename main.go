package main

import (
	"os"

	"github.com/drillmeasure/drillmeasure/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

