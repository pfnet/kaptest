package cmd

import (
	"fmt"

	"github.com/pfnet/kaptest/internal/tester"
	"github.com/spf13/cobra"
)

func newInitCmd(cfg tester.CmdConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "init [path to admission policy file]",
		Short: "Initialize the test directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("path is required")
			}
			targetFilePath := args[0]
			return tester.RunInit(cfg, targetFilePath)
		},
	}
}
