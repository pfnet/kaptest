package cmd

import (
	"fmt"
	"kaptest/internal/tester"

	"github.com/spf13/cobra"
)

func newRunCmd(cfg tester.CmdConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "run [path to test manifest]",
		Short: "Run the tests of ValidatingAdmissionPolicy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("path is required")
			}
			manifestPath := args[0]
			return tester.Run(cfg, manifestPath)
		},
	}
}
