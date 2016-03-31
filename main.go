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
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"reflect"

	"runtime/debug"
)

func main() {
	path := "."

	fileSet := token.NewFileSet()

	pkgs, err := parser.ParseDir(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
		return
	}

	files := []*ast.File{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			files = append(files, file)
		}
	}

	info := types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}

	config := &types.Config{
		Error:    func(err error) { log.Println(err) },
		Importer: importer.Default(),
	}

	pkg, err := config.Check(path, fileSet, files, &info) // Run the type checker.
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("types.config.Check(): %v\n", pkg.String())

	for _, file := range files {
		ast.Walk(&PrintASTVisitor{depth: 0, info: &info}, file)
	}
}

type PrintASTVisitor struct {
	depth int
	info  *types.Info
}

func (v *PrintASTVisitor) Visit(node ast.Node) ast.Visitor {
	if node != nil {
		for i := 0; i < v.depth; i++ {
			fmt.Print(" ")
		}

		fmt.Printf("%s", reflect.TypeOf(node).String())

		switch node.(type) {
		case ast.Expr:
			t := v.info.TypeOf(node.(ast.Expr))
			if t != nil {
				fmt.Printf(" : %s", t.String())
			}
		}

		fmt.Println()
	}

	return &PrintASTVisitor{depth: v.depth+1, info: v.info}
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
	gostack()
	gostack()
	f()
	gostack()
}

func gostack() string {
	return string(debug.Stack())
}

func noop(x interface{}) {
}

func noopChanBool(x chan bool) chan bool {
	return x
}

func f() {
	c := make(chan bool)
	go gostack()
	gostack()
	go gostack()
	gostack()

	d := c

	for flg := range noopChanBool(d) {
		noop(flg)
	}

	gostack()
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

// know that a goroutine is sending/receiving/selecting
// know the time
// know the channel (its len() and cap())
// can generate a msg-id
//
// so, can figure out what's on the chan, by msg-id.
// can figure out which goroutines are waiting on what chan's.
// can figure out which goroutines are waiting to write to a chan.
