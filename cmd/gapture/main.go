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

	"github.com/couchbaselabs/gapture/convert"
)

type Flags struct {
	Instrument string
	Tags       string
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

	s(&flags.Instrument,
		[]string{"instrument", "i"}, "PKGS", "",
		"optional, comma-separated additional packages to instrument")

	s(&flags.Tags,
		[]string{"tags"}, "TAGS]", "",
		"optional, space-separated build tags")

	i(&flags.Verbose,
		[]string{"verbose", "v"}, "INT]", 0,
		"optional, verbose logging level")
}

func usage() {
	fmt.Println(
		`gapture - tool for goroutine runtime behavior capture

Usage: gapture COMMAND [OPTIONS]

Supported COMMAND's:
- build
- help`)

	os.Exit(2)
}

func main() {
	if len(os.Args) <= 1 {
		usage()
	}

	switch os.Args[1] {
	case "help":
		usage()
	case "build":
		cmdBuild(os.Args[2:])
	default:
		usage()
	}
}

func cmdBuild(args []string) {
	flagSet.Parse(args)

	paths := flagSet.Args()
	if len(paths) <= 0 {
		paths = []string{"."}
	}

	config := NewLoaderConfig(strings.Split(flags.Tags, " "))
	if len(paths) == 1 && !strings.HasSuffix(paths[0], ".go") {
		config.Import(paths[0])
	}

	options := convert.Options{
		OnError: func(err error) { log.Println(err) },
		Logf:    MakeLogf(flags.Verbose),
	}

	err := convert.ProcessDirs(paths, options)
	if err != nil {
		log.Fatal(err)
	}
}

func NewLoaderConfig(tags []string) loader.Config {
	b := build.Default
	b.BuildTags = append(b.BuildTags, tags...)

	config := loader.Config{}
	config.Build = &b

	return config
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
