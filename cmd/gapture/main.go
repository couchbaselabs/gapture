//  Copyright (c) 2014 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"go/build"

	"golang.org/x/tools/go/loader"

	"github.com/kisielk/gotool"

	"github.com/couchbaselabs/gapture/convert"
)

type Cmd struct {
	Handler func([]string)
	Descrip string
}

var Cmds = map[string]Cmd{}

type Flags struct {
	BuildTags  string
	Help       bool
	Instrument string
	Test       bool
	Verbose    int
}

var flags Flags
var flagSet flag.FlagSet

func init() {
	s := func(v *string, names []string, kind string,
		defaultVal, usage string) { // String cmd-line param.
		for _, name := range names {
			flagSet.StringVar(v, name, defaultVal, usage)
		}
	}

	i := func(v *int, names []string, kind string,
		defaultVal int, usage string) { // Integer cmd-line param.
		for _, name := range names {
			flagSet.IntVar(v, name, defaultVal, usage)
		}
	}

	b := func(v *bool, names []string, kind string,
		defaultVal bool, usage string) { // Bool cmd-line param.
		for _, name := range names {
			flagSet.BoolVar(v, name, defaultVal, usage)
		}
	}

	s(&flags.BuildTags,
		[]string{"buildTags"}, "BUILD_TAGS]", "",
		"optional, space-separated build tags")

	b(&flags.Help,
		[]string{"help", "h", "?"}, "", false,
		"print this help message and exit")

	s(&flags.Instrument,
		[]string{"instrument", "i"}, "PKGS", "",
		"optional, comma-separated additional packages to instrument")

	b(&flags.Test,
		[]string{"test"}, "", false,
		"include the package's tests in the instrumentation")

	i(&flags.Verbose,
		[]string{"verbose", "v"}, "INT]", 0,
		"optional, verbose logging level")

	// ------------------------------------------

	Cmds["build"] = Cmd{
		CmdBuild,
		"build the instrumented code",
	}

	Cmds["help"] = Cmd{
		CmdHelp,
		"print this help message and exit",
	}
}

// ---------------------------------------------

func main() {
	cmdName := "help"
	if len(os.Args) >= 1 {
		cmdName = os.Args[1]
	}

	cmd, exists := Cmds[cmdName]
	if !exists {
		cmd = Cmds["help"]
	}

	cmd.Handler(os.Args[2:])
}

// ---------------------------------------------

func CmdHelp(args []string) {
	fmt.Println(
		`gapture - tool for goroutine runtime behavior capture

Usage: gapture CMD [OPTIONS]

Supported CMD's:`)

	for cmdName, cmd := range Cmds {
		fmt.Printf("  %s - %s\n", cmdName, cmd.Descrip)
	}

	os.Exit(2)
}

// ---------------------------------------------

func CmdBuild(args []string) {
	flagSet.Parse(args)

	if flags.Help {
		CmdHelp(args)
		return
	}

	config := loader.Config{}

	if flags.BuildTags != "" {
		config.Build = &build.Default
		config.Build.BuildTags = append(config.Build.BuildTags,
			strings.Split(flags.BuildTags, " ")...)
	}

	argsRest, err := config.FromArgs(flagSet.Args(), flags.Test)
	if err != nil {
		log.Fatal(err)
		return
	}

	instrument := strings.Trim(flags.Instrument, ", ")
	if instrument != "" {
		for _, pkg := range strings.Split(instrument, ",") {
			// Expands "..." wildcards.
			for _, path := range gotool.ImportPaths([]string{pkg}) {
				config.Import(path)
			}
		}
	}

	prog, err := config.Load()
	if err != nil {
		log.Fatal(err)
		return
	}

	options := convert.Options{
		OnError: func(err error) { log.Println(err) },
		Logf:    MakeLogf(flags.Verbose),
	}

	_ = argsRest // TODO.

	err = convert.ProcessProgram(prog, options)
	if err != nil {
		log.Fatal(err)
	}
}

func MakeLogf(level int) func(fmt string, v ...interface{}) {
	return func(fmt string, v ...interface{}) {
		spaces := 0
		for {
			if fmt[spaces] != ' ' {
				break
			}
			spaces++
		}

		if level > spaces {
			log.Printf(fmt, v...)
		}
	}
}
