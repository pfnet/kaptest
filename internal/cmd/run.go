package cmd

import (
	"kaptest/internal/tester"

	"github.com/spf13/cobra"
)

func newRunCmd(cfg tester.CmdConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "run [path to kaptest.yaml]",
		Short: "Run the tests of ValidatingAdmissionPolicy",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tester.Run(cmd, args, cfg)
		},
	}
}
