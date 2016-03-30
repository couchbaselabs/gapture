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
	"log"
	"go/ast"
	"go/parser"
	"go/token"
	"runtime"
)

func main() {
	fileSet := token.NewFileSet()

	path := "."

	pkgs, err := parser.ParseDir(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		log.Fatalf("%v", err)
		return
	}

	log.Printf("fileSet: %#v", fileSet)

	for pkgName, pkg := range pkgs {
		log.Printf("%s => %#v", pkgName, pkg)
		for fileName, file := range pkg.Files {
			ConvertFile(fileSet, fileName, file)
		}
	}
}

func ConvertFile(fileSet *token.FileSet, fName string, f *ast.File) error {
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			ast.Print(fileSet, x)
		case *ast.RangeStmt:
			ast.Print(fileSet, n)
		case *ast.CommClause:
			ast.Print(fileSet, n)
		case *ast.SendStmt:
			ast.Print(fileSet, n)
		case *ast.UnaryExpr:
			if x.Op == token.ARROW { // The receive "<-" operator.
				ast.Print(fileSet, x)
			}
		}
		return true
	})

	return nil
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
		noop(c); c <- true; noop(1 + 2)
	}
}

func noop(x interface{}) {
}

func noopChanBool(x chan bool) chan bool {
	return x
}

func f() {
	c := make(chan bool)
	go gostack(c)
	gostack(nil)
	go gostack(c)
	gostack(nil)

	d := c

	for flg := range noopChanBool(d) {
		noop(flg)
	}

	gostack(nil)
}

// x, ok := <-c
// x := <-c
// <-c || whatever
//    expression based won't work because we don't know return type!
//      AFTER(c, <-BEFORE(c))
//      RECV(c)

// c <- 123
// c <- 1 + 2
//    expression based won't work because we don't know return type!
//      c <- BEFORE(c, 1 + 2); AFTER(c)
//      SEND(c, 1 + 2)
//    statement conversion...
 //      var gen_sym_123 := 1 + 2
//      gapture.BeforeSend(c, gen_sym, "1 + 2")
//      c <- gen_sym_123
//      gapture.AfterSend(c, gen_sym, "1 + 2")
