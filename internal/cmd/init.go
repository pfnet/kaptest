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
	"fmt"

	"github.com/pfnet/kaptest/internal/tester"
	"github.com/spf13/cobra"
)

func newInitCmd(cfg tester.CmdConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "init [path to admission policy file]",
		Short: "Generate skeleton manifests for writing tests",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("path is required")
			}
			targetFilePath := args[0]
			return tester.RunInit(cfg, targetFilePath)
		},
	}
}
