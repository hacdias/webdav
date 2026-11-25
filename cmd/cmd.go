package cmd

import (
	"os"
)

// Execute executes the commands.
func Execute() {
	// Cobra already printed the error.
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
