package tester

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const KAPTEST_DIR = ".kaptest/"

func RunInit(cmd *cobra.Command, args []string, cfg CmdConfig) error {
	dir := args[0]
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	file, err := os.Stat(dir + KAPTEST_DIR)
	if err != nil {
		dirMode := dirInfo.Mode()
		perm := dirMode & os.ModePerm
		if err := os.Mkdir(dir+KAPTEST_DIR, perm); err != nil {
			return fmt.Errorf("failed to make dir: %w", err)
		}
	} else if !file.IsDir() {
		return fmt.Errorf("file %s already exists", dir+KAPTEST_DIR)
	}
	if file, err := os.Stat(dir + KAPTEST_DIR); err != nil || !file.IsDir() {
		dirMode := dirInfo.Mode()
		perm := dirMode & os.ModePerm
		if err := os.Mkdir(dir+KAPTEST_DIR, perm); err != nil {
			return fmt.Errorf("failed to make dir: %w", err)
		}
	}

	f, err := os.Create(dir + KAPTEST_DIR + "kaptest.yaml")
	if err != nil {
		return fmt.Errorf("failed to create kaptest.yaml: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(defaultManifest); err != nil {
		return fmt.Errorf("failed to write in kaptest.yaml: %w", err)
	}

	_, err = os.Create(dir + KAPTEST_DIR + "resources.yaml")
	if err != nil {
		return fmt.Errorf("failed to create resources.yaml: %w", err)
	}
	return nil
}

var defaultManifest = `validatingAdmissionPolicies:
  - # policy.yaml
resources:
  - resources.yaml
testSuites:
  - policy: # policy-name
    tests:
      - object:
          kind: Deployment
          name: good-deployment
        expect: admit
      - object:
          kind: Deployment
          name: bad-deployment
        expect: deny
`
