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

// Package convert provides tooling API's to instrument go code with
// calls to gapture's runtime API.
package convert

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
)

var GapturePackageName = "github.com/couchbaselabs/gapture"

// Options allows users to override the default behavior of the
// instrumentation processing.
type Options struct {
	TokenFileSet *token.FileSet
	TypesInfo    *types.Info
	TypesConfig  *types.Config

	OnError func(error)
	Logf    func(fmt string, v ...interface{})
}

var GaptureStackAssignStmt *ast.AssignStmt

func init() {
	expr, err := parser.ParseExpr(`func() { gaptureGID, gaptureStack := gapture.Stack(0) }`)
	if err != nil {
		panic(err)
	}
	GaptureStackAssignStmt = expr.(*ast.FuncLit).Body.List[0].(*ast.AssignStmt)
	fmt.Printf("GaptureStackAssignStmt: %#v\n", GaptureStackAssignStmt)
}

func ProcessDirs(paths []string, options Options) error {
	logf := options.Logf
	if logf == nil {
		logf = func(fmt string, v ...interface{}) { /* noop */ }
	}

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

			for fileName, file := range pkg.Files {
				// If the file defines func's, then add import of
				// gapture package, if not already.
				if FileHasFuncs(file) &&
					!FileImportsPackage(file, GapturePackageName) {
					file.Decls = append([]ast.Decl{
						&ast.GenDecl{
							Tok: token.IMPORT,
							Specs: []ast.Spec{
								&ast.ImportSpec{
									Path: &ast.BasicLit{
										Kind:  token.STRING,
										Value: `"` + GapturePackageName + `"`,
									},
								},
							},
						},
					}, file.Decls...)
				}

				ast.Walk(&Converter{
					options:  &options,
					info:     info,
					fileName: fileName,
					file:     file,
					node:     file,
				}, file)

				err = format.Node(os.Stdout, fileSet, file)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}

	return nil
}

// ----------------------------------------------------------------

// FileImportsPackage returns true if a file imports a given pkgName.
func FileImportsPackage(file *ast.File, pkgName string) bool {
	pkgNameDQ := `"` + pkgName + `"`

	for _, importSpec := range file.Imports {
		if importSpec != nil &&
			importSpec.Path != nil &&
			(importSpec.Path.Value == pkgName || importSpec.Path.Value == pkgNameDQ) {
			return true
		}
	}

	return false
}

func FileHasFuncs(file *ast.File) bool {
	for _, decl := range file.Decls {
		_, ok := decl.(*ast.FuncDecl)
		if ok {
			return true
		}
	}

	return false
}

// ----------------------------------------------------------------

type Converter struct {
	options  *Options
	info     *types.Info
	fileName string
	file     *ast.File
	node     ast.Node
	parent   *Converter
}

func (v *Converter) Visit(node ast.Node) ast.Visitor {
	if node != nil {
		fmt.Printf("%s ", v.fileName)

		for vv := v; vv != nil; vv = vv.parent {
			fmt.Print(" ")
		}

		fmt.Printf("%s", reflect.TypeOf(node).String())

		switch x := node.(type) {
		case *ast.File:
			fmt.Printf(" name: %v", x.Name)
		case *ast.FuncDecl:
			fmt.Printf(" name: %v", x.Name)
		case *ast.BasicLit:
			fmt.Printf(" value: %v", x.Value)
		case ast.Expr:
			t := v.info.TypeOf(x)
			if t != nil {
				fmt.Printf(" : %s", t.String())
			}
		}

		fmt.Println()
	}

	return &Converter{
		options:  v.options,
		info:     v.info,
		fileName: v.fileName,
		file:     v.file,
		node:     node,
		parent:   v,
	}
}
