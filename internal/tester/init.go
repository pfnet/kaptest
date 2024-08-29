package tester

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

const (
	rootManifestName     = "kaptest.yaml"
	resourceManifestName = "resources.yaml"
)

func RunInit(cmd *cobra.Command, args []string, cfg CmdConfig) error {
	if len(args) == 0 {
		return fmt.Errorf("filepath is required")
	}
	targetFilePath := args[0]
	if err := createTestDir(targetFilePath); err != nil {
		return fmt.Errorf("create test directory: %w", err)
	}
	if err := createTestManifests(targetFilePath); err != nil {
		return fmt.Errorf("create test manifests: %w", err)
	}

	fmt.Printf("Test dir is created at %q.\n", testDir(targetFilePath))
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

func createTestManifests(targetFilePath string) error {
	dir := testDir(targetFilePath)
	f, err := os.Create(filepath.Join(dir, rootManifestName))
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			slog.Info(fmt.Sprintf("file already exists: %s", rootManifestName))
			return nil
		}
		return fmt.Errorf("create %s: %w", rootManifestName, err)
	}
	defer f.Close()

	targetPolicyNames, err := getPolicyNames(targetFilePath)
	if err != nil {
		return fmt.Errorf("get test target policies: %w", err)
	}
	slog.Debug(fmt.Sprintf("test target policies: %v", targetPolicyNames))

	fileName := filepath.Base(targetFilePath)
	manifestBuf := baseManifest(fileName, targetPolicyNames)
	if _, err := f.Write(manifestBuf); err != nil {
		return fmt.Errorf("write in %s: %w", rootManifestName, err)
	}

	_, err = os.Create(filepath.Join(dir, resourceManifestName))
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			slog.Info(fmt.Sprintf("file already exists: %s", resourceManifestName))
			return nil
		}
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
							Kind: "Pod",
						},
						NamespacedName: NamespacedName{
							Name: "ok",
						},
					},
					Expect: validating.EvalAdmit,
				},
				{
					Object: NameWithGVK{
						GVK: GVK{
							Kind: "Pod",
						},
						NamespacedName: NamespacedName{
							Name: "bad",
						},
					},
					Expect: validating.EvalDeny,
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
			slog.Warn("failed to decode ValidatingAdmissionPolicy", "error", err)
			continue
		}
		if vap.Kind != "ValidatingAdmissionPolicy" {
			continue
		}
		policyNames = append(policyNames, vap.Name)
	}
	return policyNames, nil
}
