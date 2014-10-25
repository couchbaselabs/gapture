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
	"fmt"
	"go/parser"
	"go/token"
	"go/ast"
	"runtime"
)

func main() {
	fset := token.NewFileSet() // positions are relative to fset

	// Parse the file containing this very example
	// but stop after processing the imports.
	f, err := parser.ParseFile(fset, "main.go", nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(fset)

	// Print the imports from the file's AST.
	for _, s := range f.Imports {
		fmt.Println(s.Path.Value)
	}

	m := map[ast.Node]bool{}

	ast.Inspect(f, func(n ast.Node) bool {
		m[n] = true
		var s string
		switch x := n.(type) {
		case *ast.BasicLit:
			s = x.Value
		case *ast.Ident:
			s = x.Name
		}
		if s != "" {
			fmt.Printf("%s:\t%s\n", fset.Position(n.Pos()), s)
		} else {
			fmt.Printf(".\n")
		}

		return true
	})

	// Print the AST.
	ast.Print(fset, f)
}

func mainPrev() {
	gostack(nil)
	gostack(nil)
	f()
	gostack(nil)
}

func gostack(c chan bool) {
	buf := make([]byte, 6000000)
	n := runtime.Stack(buf, false)

	fmt.Println("current:", string(buf[0:n]))
	if c != nil {
		c <- true
	}
}

func f() {
	c := make(chan bool)
	go gostack(c)
	gostack(nil)
	<-c
	go gostack(c)
	gostack(nil)
	<-c
	gostack(nil)
}
