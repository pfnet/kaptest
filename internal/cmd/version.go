package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "v0.0.0"
	commit  = "HEAD"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			return nil
		},
	}
}
