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
)

// Root function to execute commands
func main() {
	cfg := tester.CliConfig{}
	flag.BoolVar(&cfg.Debug, "debug", false, "Debug output")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")
	flag.Parse()
	cfg.ManifestPath = flag.Arg(0)
	if cfg.ManifestPath == "" {
		usage()
		return
	}

	initLog(cfg)

	if err := tester.Run(cfg); err != nil {
		if errors.Is(err, tester.ErrTestFail) {
			os.Exit(1)
		}
		log.Fatalf("Error: %v", err)
	}
}

func usage() {
	fmt.Println("Usage: file <file>")
}

func initLog(cfg tester.CliConfig) {
	if cfg.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
}
