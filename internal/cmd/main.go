// Copyright 2024 Preferred Networks, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"flag"
	"fmt"
	"kaptest/internal/tester"
	"log"
	"log/slog"
	"os"
	"strings"
)

type flags struct {
	Debug   bool
	Verbose bool
}

// Root function to execute commands
func main() {
	cfg := flags{}
	flag.BoolVar(&cfg.Debug, "debug", false, "Debug output")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")
	flag.Parse()
	command := flag.Arg(0)
	initLog(cfg)

	switch command {
	case "run":
		execRun(cfg)
	case "init":
		execInit(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

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

func execRun(flags flags) {
	cfg := tester.CliConfig{
		Debug:   flags.Debug,
		Verbose: flags.Verbose,
	}
	cfg.ManifestPath = flag.Arg(1)
	if cfg.ManifestPath == "" {
		usage()
		return
	}

	if err := tester.Run(cfg); err != nil {
		if errors.Is(err, tester.ErrTestFail) {
			os.Exit(1)
		}
		log.Fatalf("Error: %v", err)
	}
}

func usage() {
	fmt.Printf(`Usage
  init <dir> : setup kaptest in <dir>
  run <file> : run test
	`)
}

func initLog(flags flags) {
	if flags.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}
