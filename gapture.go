//  Copyright (c) 2016 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package gapture

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"runtime"
)

type Options struct {
	TokenFileSet *token.FileSet
	TypesInfo    *types.Info
	TypesConfig  *types.Config

	OnError func(error)
	Logf    func(fmt string, v ...interface{})
}

func ProcessDirs(paths []string, options Options) error {
	logf := options.Logf

	fileSet := options.TokenFileSet
	if fileSet == nil {
		fileSet = token.NewFileSet()
	}

	info := options.TypesInfo
	if info == nil {
		info = &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{},
			Defs:  map[*ast.Ident]types.Object{},
			Uses:  map[*ast.Ident]types.Object{},
		}
	}

	config := options.TypesConfig
	if config == nil {
		config = &types.Config{
			Error:    options.OnError,
			Importer: importer.Default(),
		}
	}

	for _, path := range paths {
		pkgs, err := parser.ParseDir(fileSet, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		for _, pkg := range pkgs {
			files := []*ast.File{}
			for _, file := range pkg.Files {
				files = append(files, file)
			}

			pkgChecked, err := config.Check(pkg.Name, fileSet, files, info)
			if err != nil {
				return err
			}

			logf("types.config.Check(): %s => %v\n", pkg.Name, pkgChecked)

			for _, file := range pkg.Files {
				ast.Walk(&PrintASTVisitor{
					options: &options,
					depth: 0,
					info: info,
				}, file)

				format.Node(os.Stdout, fileSet, file)
			}
		}
	}

	return nil
}

type PrintASTVisitor struct {
	options *Options
	depth   int
	info    *types.Info
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

	return &PrintASTVisitor{options: v.options, depth: v.depth+1, info: v.info}
}

func Stack() string {
	buf := make([]byte, 6000000)
	n := runtime.Stack(buf, false)
	return string(buf[0:n])
}

// Return string of Stack() looks like the following (and has "\t" tabs)...
//
// goroutine 1 [running]:
// main.gostack(0x0, 0x0)
//     /Users/steveyen/go/src/github.com/couchbaselabs/gapture/main.go:128 +0x7b
// main.main()
//     /Users/steveyen/go/src/github.com/couchbaselabs/gapture/main.go:27 +0x27

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

// ------------------------------------------------------------
// Statement/expression:      Does it need markup/rewrite?
//
//                            beg end bodyBeg bodyEnd
// go funcExpr(...)           n   n
//
// close(chExpr)              y   y
//
// chExpr <- msgExpr          y   y
//
// <-chExpr                   y   y
//
// for range chExpr { ... }   y   y   y       y
//
// select {                   y   y+ (every caseStmt and default)
//    case sendOrRecvExpr:
//    default:
// }
//
// cgo call                   y   y
//
// panic(...)                 n   n


