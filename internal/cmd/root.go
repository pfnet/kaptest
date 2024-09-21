/*
Copyright 2024 Preferred Networks, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"log/slog"
	"os"

	"github.com/pfnet/kaptest/internal/tester"
	"github.com/spf13/cobra"
)

func Execute() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "kaptest",
		Short:        "Kubernetes Admission Policy TESTing tool",
		SilenceUsage: true,
	}
	cfg := tester.CmdConfig{}
	cmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "v", false, "Verbose output")
	cmd.PersistentFlags().BoolVarP(&cfg.Debug, "debug", "d", false, "Debug output")

	cobra.OnInitialize(func() {
		initLog(cfg)
	})

	cmd.AddCommand(newInitCmd(&cfg))
	cmd.AddCommand(newRunCmd(&cfg))
	cmd.AddCommand(newVersionCmd())
	return cmd
}

func initLog(cfg tester.CmdConfig) {
	if cfg.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}
