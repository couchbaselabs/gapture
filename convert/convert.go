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
// calls to the runtime API.
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
	"strings"
)

var RuntimePackageName = "github.com/couchbaselabs/gapture"

// Options allows users to override the default behavior of the
// instrumentation processing.
type Options struct {
	TokenFileSet *token.FileSet
	TypesInfo    *types.Info
	TypesConfig  *types.Config

	OnError func(error)
	Logf    func(fmt string, v ...interface{})
}

var RuntimeStackAssignStmt *ast.AssignStmt

func init() {
	expr, err :=
		parser.ParseExpr(`func() { gaptureGID, gaptureStack := gapture.Stack(0) }`)
	if err != nil {
		panic(err)
	}
	RuntimeStackAssignStmt = expr.(*ast.FuncLit).Body.List[0].(*ast.AssignStmt)
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
				converter := &Converter{
					options:  &options,
					info:     info,
					fileName: fileName,
					file:     file,
					node:     file,
				}

				ast.Walk(converter, file)

				// If the file had conversions, then add import of the
				// runtime package, if not already.
				if converter.conversions > 0 &&
					!FileImportsPackage(file, RuntimePackageName) {
					file.Decls = append([]ast.Decl{
						&ast.GenDecl{
							Tok: token.IMPORT,
							Specs: []ast.Spec{
								&ast.ImportSpec{
									Path: &ast.BasicLit{
										Kind:  token.STRING,
										Value: `"` + RuntimePackageName + `"`,
									},
								},
							},
						},
					}, file.Decls...)
				}

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

// UsesChannels returns true if the ast.Node uses channels.
func UsesChannels(info *types.Info, topNode ast.Node) bool {
	rv := false

	ast.Inspect(topNode, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.SelectStmt:
			rv = true
		case *ast.SendStmt:
			rv = true
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				rv = true
			}
		case *ast.RangeStmt:
			t := info.TypeOf(x.X)
			rv = rv || strings.HasPrefix(t.String(), "chan ")
		}

		return rv == false
	})

	return rv
}

// ----------------------------------------------------------------

type Converter struct {
	parent   *Converter
	options  *Options
	info     *types.Info
	fileName string
	file     *ast.File
	node     ast.Node

	conversions int
}

func (v *Converter) MarkConversion() {
	if v == nil {
		return
	}

	v.conversions++
	v.parent.MarkConversion()
}

func (v *Converter) Visit(node ast.Node) ast.Visitor {
	if node != nil {
		fmt.Printf("%s ", v.fileName)

		for vv := v; vv != nil; vv = vv.parent { // Indentation by depth.
			fmt.Print(" ")
		}

		fmt.Printf("%s", reflect.TypeOf(node).String())

		switch x := node.(type) {
		case *ast.FuncDecl:
			fmt.Printf(" name: %v", x.Name)
			if UsesChannels(v.info, x) {
				fmt.Printf(" uses-channels")
			}
		case *ast.FuncLit:
			if UsesChannels(v.info, x) {
				fmt.Printf(" uses-channels")
			}
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
		parent:   v,
		options:  v.options,
		info:     v.info,
		fileName: v.fileName,
		file:     v.file,
		node:     node,
	}
}
