package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const KAPTEST_DIR = ".kaptest/"

var defaultManifest = `validatingAdmissionPolicies:
  -  # policy.yaml
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

func execInit(_ flags) {
	dir := flag.Arg(1)
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		panic(err)
	}
	file, err := os.Stat(dir + KAPTEST_DIR)
	if err != nil {
		dirMode := dirInfo.Mode()
		perm := dirMode & os.ModePerm
		if err := os.Mkdir(dir+KAPTEST_DIR, perm); err != nil {
			panic(fmt.Errorf("failed to make dir: %v", err))
		}
	} else if !file.IsDir() {
		panic(fmt.Errorf("file %s already exists", dir+KAPTEST_DIR))
	}
	if file, err := os.Stat(dir + KAPTEST_DIR); err != nil || !file.IsDir() {
		dirMode := dirInfo.Mode()
		perm := dirMode & os.ModePerm
		if err := os.Mkdir(dir+KAPTEST_DIR, perm); err != nil {
			panic(fmt.Errorf("failed to make dir: %v", err))
		}
	}

	f, err := os.Create(dir + KAPTEST_DIR + "kaptest.yaml")
	if err != nil {
		panic(fmt.Errorf("failed to create kaptest.yaml: %v", err))
	}
	defer f.Close()

	f.WriteString(defaultManifest)
}
