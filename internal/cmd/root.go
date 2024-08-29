package cmd

import (
	"kaptest/internal/tester"
	"log"
	"log/slog"

	"github.com/spf13/cobra"
)

func Execute() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kaptest",
		Short:        "Kubernetes Admission Policy Tester",
		SilenceUsage: true,
	}
	cfg := tester.CmdConfig{}
	cmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().BoolVarP(&cfg.Debug, "debug", "d", false, "Debug output")

	initLog(cfg)

	cmd.AddCommand(newInitCmd(cfg))
	cmd.AddCommand(newRunCmd(cfg))
	return cmd
}

func initLog(cfg tester.CmdConfig) {
	if cfg.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}
