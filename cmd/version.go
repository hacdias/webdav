package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "(untracked)"

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("WebDAV version " + version)
		},
	})
}
