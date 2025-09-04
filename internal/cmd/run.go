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
	"os"
	"path/filepath"

	"github.com/pfnet/kaptest/internal/tester"
	"github.com/spf13/cobra"
)

func newRunCmd(cfg *tester.CmdConfig) *cobra.Command {
	testerCfg := tester.TesterCmdConfig{
		CmdConfig: *cfg,
	}
	cmd := &cobra.Command{
		Use:   "run [path to test manifest]...",
		Short: "Run the tests of ValidatingAdmissionPolicy and MutatingAdmissionPolicy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("path is required")
			}
			return tester.Run(testerCfg, args)
		},
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	cmd.Flags().BoolVarP(&testerCfg.ValidateResourceManifest, "validate-resource-manifests", "", true, "Validating the resource manifests according to the schema")
	cmd.Flags().StringVarP(&testerCfg.SchemaCache, "schema-cache", "", filepath.Join(homeDir, ".cache/kaptest/schema"), "Path to cache schemas used in resource manifest validation")

	return cmd
}
