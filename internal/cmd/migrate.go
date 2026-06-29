/*
Copyright 2026 Preferred Networks, Inc.

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

	"github.com/pfnet/kaptest/internal/tester"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// legacyTestManifests represents the TestManifests format used in v0.1.x .
type legacyTestManifests struct {
	ValidatingAdmissionPolicies []string                     `yaml:"validatingAdmissionPolicies,omitempty"`
	Resources                   []string                     `yaml:"resources,omitempty"`
	TestSuites                  []legacyTestsForSinglePolicy `yaml:"testSuites,omitempty"`
}

type legacyTestsForSinglePolicy struct {
	Policy string           `yaml:"policy"`
	Tests  []legacyTestCase `yaml:"tests"`
}

type legacyTestCase struct {
	Object    tester.NameWithGVK          `yaml:"object,omitempty"`
	OldObject tester.NameWithGVK          `yaml:"oldObject,omitempty"`
	Param     tester.NamespacedName       `yaml:"param,omitempty"`
	Expect    tester.PolicyDecisionExpect `yaml:"expect,omitempty"`
	UserInfo  tester.UserInfo             `yaml:"userInfo,omitempty"`
}

func newMigrateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate [path to test manifest]...",
		Short: "Migrate test manifests from v0.1.x format to the v1alpha1 format",
		Long: `Migrate test manifests written in the v0.1.x format to the v1alpha1 format.

The following changes are applied:
  - Add "version: v1alpha1"
  - Rename "validatingAdmissionPolicies" to "policies"
  - Rename "testSuites" to "vapTestSuites"

Files that already contain a "version" field are skipped.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("path is required")
			}
			for _, path := range args {
				if err := migrateManifestFileFromv0_1_xTov1alpha1(path, dryRun); err != nil {
					return fmt.Errorf("migrate %s: %w", path, err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the migrated manifest to stdout without writing to file")
	return cmd
}

func migrateManifestFileFromv0_1_xTov1alpha1(path string, dryRun bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Skip files that are already in a versioned format.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if _, ok := raw["version"]; ok {
		fmt.Printf("skip (already versioned): %s\n", path)
		return nil
	}

	var legacy legacyTestManifests
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	newManifest := tester.TestManifests{
		Version:   "v1alpha1",
		Policies:  legacy.ValidatingAdmissionPolicies,
		Resources: legacy.Resources,
	}
	for _, suite := range legacy.TestSuites {
		vapSuite := tester.TestsForSingleVapPolicy{Policy: suite.Policy}
		for _, tc := range suite.Tests {
			vapSuite.Tests = append(vapSuite.Tests, tester.VAPTestCase{
				Object:    tc.Object,
				OldObject: tc.OldObject,
				Param:     tc.Param,
				Expect:    tc.Expect,
				UserInfo:  tc.UserInfo,
			})
		}
		newManifest.VapTestSuites = append(newManifest.VapTestSuites, vapSuite)
	}

	out, err := yaml.Marshal(newManifest)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if dryRun {
		fmt.Printf("# %s\n%s\n", path, string(out))
		return nil
	}

	if err := os.WriteFile(path, out, 0o644); err != nil { //nolint:gosec
		return err
	}
	fmt.Printf("migrated: %s\n", path)
	return nil
}
