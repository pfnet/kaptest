package cmd

import (
	"kaptest/internal/tester"

	"github.com/spf13/cobra"
)

func newInitCmd(cfg tester.CmdConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "init [dir]",
		Short: "Initialize the test directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tester.RunInit(cmd, args, cfg)
		},
	}
}
