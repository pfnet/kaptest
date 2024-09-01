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

package tester

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	rootManifestName     = "kaptest.yaml"
	resourceManifestName = "resources.yaml"
)

func RunInit(cfg CmdConfig, targetFilePath string) error {
	if err := createTestDir(targetFilePath); err != nil {
		return fmt.Errorf("create test directory: %w", err)
	}
	if err := createRootManifest(targetFilePath); err != nil {
		return fmt.Errorf("create root manifests: %w", err)
	}
	if err := createResourceManifest(targetFilePath); err != nil {
		return fmt.Errorf("create resource manifests: %w", err)
	}

	fmt.Printf("Test dir is generated at %q.\n", testDir(targetFilePath))
	return nil
}

func createTestDir(targetFilePath string) error {
	if _, err := os.Stat(targetFilePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("admission policy file is not found")
		}
		return fmt.Errorf("check file: %w", err)
	}

	dirInfo, err := os.Stat(filepath.Dir(targetFilePath))
	if err != nil {
		return fmt.Errorf("get parent directory info: %w", err)
	}

	dir := testDir(targetFilePath)
	if err := os.Mkdir(dir, dirInfo.Mode()&os.ModePerm); err != nil {
		if errors.Is(err, os.ErrExist) {
			slog.Info(fmt.Sprintf("directory already exists: %s", dir))
		} else {
			return fmt.Errorf("make dir: %w", err)
		}
	}

	return nil
}

func createRootManifest(targetFilePath string) error {
	dir := testDir(targetFilePath)
	p := filepath.Join(dir, rootManifestName)

	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("file already exists: %s", p)
	}

	targetPolicyNames, err := getPolicyNames(targetFilePath)
	if err != nil {
		return fmt.Errorf("get test target policies: %w", err)
	}
	slog.Debug(fmt.Sprintf("test target policies: %v", targetPolicyNames))

	fileName := filepath.Base(targetFilePath)
	manifestBuf := baseManifest(fileName, targetPolicyNames)

	if err := os.WriteFile(p, manifestBuf, 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("create %s: %w", rootManifestName, err)
	}

	return nil
}

func createResourceManifest(targetFilePath string) error {
	dir := testDir(targetFilePath)
	p := filepath.Join(dir, resourceManifestName)

	if _, err := os.Stat(p); err == nil {
		return fmt.Errorf("file already exists: %s", p)
	}
	if _, err := os.Create(p); err != nil {
		return fmt.Errorf("create %s: %w", resourceManifestName, err)
	}

	return nil
}

func testDir(targetFilePath string) string {
	return targetFilePath[:len(targetFilePath)-len(filepath.Ext(targetFilePath))] + ".test"
}

func baseManifest(targetPath string, policies []string) []byte {
	m := TestManifests{
		ValidatingAdmissionPolicies: []string{filepath.Join("..", targetPath)},
		Resources:                   []string{resourceManifestName},
		TestSuites:                  []TestsForSinglePolicy{},
	}
	for _, p := range policies {
		m.TestSuites = append(m.TestSuites, TestsForSinglePolicy{
			Policy: p,
			Tests: []TestCase{
				{
					Object: NameWithGVK{
						GVK: GVK{
							Kind: "CHANGEME",
						},
						NamespacedName: NamespacedName{
							Name: "ok",
						},
					},
					Expect: Admit,
				},
				{
					Object: NameWithGVK{
						GVK: GVK{
							Kind: "CHANGEME",
						},
						NamespacedName: NamespacedName{
							Name: "bad",
						},
					},
					Expect: Deny,
				},
			},
		})
	}
	b, _ := yaml.Marshal(m)
	return b
}

func getPolicyNames(targetFilePath string) ([]string, error) {
	yamlFile, err := os.Open(targetFilePath)
	if err != nil {
		return nil, err
	}
	decoder := kyaml.NewYAMLToJSONDecoder(yamlFile)
	var policyNames []string
	for {
		var vap v1.ValidatingAdmissionPolicy
		if err := decoder.Decode(&vap); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			slog.Warn("failed to decode", "error", err)
			continue
		}
		if vap.Kind != "ValidatingAdmissionPolicy" {
			slog.Warn("not a ValidatingAdmissionPolicy", "kind", vap.Kind)
			continue
		}
		policyNames = append(policyNames, vap.Name)
	}
	return policyNames, nil
}
