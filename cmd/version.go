package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var version = "untracked"

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			// Builds without ldflags, such as go install, still carry the module version.
			if info, ok := debug.ReadBuildInfo(); ok && version == "untracked" {
				version = info.Main.Version
			}

			fmt.Printf("WebDAV version: %s\n", version)
		},
	})
}
