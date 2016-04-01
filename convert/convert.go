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

// RuntimeFuncPrefix is an AST snippet inserted as initialization
// stmt's in rewritten func bodies, in order to declare required vars.
var RuntimeFuncPrefix []ast.Stmt

func init() {
	expr, err := parser.ParseExpr(`func() { var gaptureGCtx gapture.GCtx }`)
	if err != nil {
		panic(err)
	}
	RuntimeFuncPrefix = expr.(*ast.FuncLit).Body.List
}

// ------------------------------------------------------

// Options allows users to override the default behavior of the
// instrumentation processing.
type Options struct {
	TokenFileSet *token.FileSet
	TypesInfo    *types.Info
	TypesConfig  *types.Config

	OnError func(error)
	Logf    func(fmt string, v ...interface{})
}

// ------------------------------------------------------

// ProcessDirs instruments the code in the given directory paths.
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
					info:     info,
					fileName: fileName,
					file:     file,
					logf:     logf,
					node:     file,
				}

				ast.Walk(converter, file)

				// If the file had modifications, then add import of the
				// runtime package, if not already.
				if converter.modifications > 0 &&
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

// UsesChannels returns true if the ast.Node actively uses channels.
// That is, if the code invokes the <- operator (to send or receive),
// uses select {}, uses close(), or ranges over a chan, then the
// return value is true.  In contrast, a func that just passes through
// chan instances as parameters, but doesn't actively use them, would
// have return value of false.
func UsesChannels(info *types.Info, topNode ast.Node) bool {
	rv := false

	ast.Inspect(topNode, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.SendStmt:
			rv = true
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				rv = true
			}
		case *ast.SelectStmt:
			rv = true
		case *ast.CallExpr:
			ident, ok := x.Fun.(*ast.Ident)
			if ok && ident.Name == "close" {
				rv = true
			}
		case *ast.RangeStmt:
			t := info.TypeOf(x.X)
			rv = strings.HasPrefix(t.String(), "chan ")
		}

		return rv == false
	})

	return rv
}

// ----------------------------------------------------------------

// A Converter implements the ast.Visitor interface to instrument code
// with runtime API invocations to capture goroutine/stack info.
type Converter struct {
	parent   *Converter
	info     *types.Info
	fileName string
	file     *ast.File
	logf     func(fmt string, v ...interface{})
	node     ast.Node

	modifications int // Count of modifications made to this subtree.
}

var indent = "......................................................"

func (v *Converter) Visit(node ast.Node) ast.Visitor {
	vChild := &Converter{
		parent:   v,
		info:     v.info,
		fileName: v.fileName,
		file:     v.file,
		logf:     v.logf,
		node:     node,
	}

	if node != nil {
		depth := 0
		for vv := v; vv != nil; vv = vv.parent { // Indentation by depth.
			depth++
		}

		msg := ""

		switch x := node.(type) {
		case *ast.FuncDecl:
			msg = fmt.Sprintf(" name: %v", x.Name)
			if UsesChannels(vChild.info, x) {
				x.Body.List = InsertStmts(x.Body.List, 0, RuntimeFuncPrefix)
				vChild.MarkModified()
			}

		case *ast.FuncLit:
			if UsesChannels(vChild.info, x) {
				x.Body.List = InsertStmts(x.Body.List, 0, RuntimeFuncPrefix)
				vChild.MarkModified()
			}

		case *ast.CallExpr:
			ident, ok := x.Fun.(*ast.Ident)
			if ok && ident.Name == "close" && len(x.Args) == 1 {
				// Convert:
				//   close(chExpr)
				// Into:
				//   close(gaptureGCtx.OnChanClose(chExpr).(chan foo))
				//   gaptureGCtx.OnChanCloseDone()
				//
				x.Args = []ast.Expr{
					&ast.TypeAssertExpr{
						X: &ast.CallExpr{
							Fun: &ast.Ident{
								Name: "gaptureGCtx.OnChanClose",
							},
							Args: x.Args,
						},
						Type: &ast.Ident{
							Name: vChild.info.TypeOf(x.Args[0]).String(),
						},
					},
				}
				vChild.InsertStmtsAfter([]ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{
								Name: "gaptureGCtx.OnChanCloseDone",
							},
						},
					},
				})
				vChild.MarkModified()
			}

		case *ast.SendStmt:
			// Convert:
			//   chExpr <- msgExpr
			// Into:
			//   gaptureGCtx.OnChanSend(chExpr).(chan foo) <- msgExpr
			//   gaptureGCtx.OnChanSendDone()
			//
			x.Chan = &ast.TypeAssertExpr{
				X: &ast.CallExpr{
					Fun: &ast.Ident{
						Name: "gaptureGCtx.OnChanSend",
					},
					Args: []ast.Expr{x.Chan},
				},
				Type: &ast.Ident{
					Name: vChild.info.TypeOf(x.Chan).String(),
				},
			}
			vChild.InsertStmtsAfter([]ast.Stmt{
				&ast.ExprStmt{
					X: &ast.CallExpr{
						Fun: &ast.Ident{
							Name: "gaptureGCtx.OnChanSendDone",
						},
					},
				},
			})
			vChild.MarkModified()

		case *ast.UnaryExpr:
			// Convert:
			//   <-chExpr
			// Into:
			//   <-gaptureCtx.OnChanRecv(chExpr).(chan foo)
			//   gaptureCtx.OnChanRecvDone()
			//
			if x.Op == token.ARROW {
				x.X = &ast.TypeAssertExpr{
					X: &ast.CallExpr{
						Fun: &ast.Ident{
							Name: "gaptureGCtx.OnChanRecv",
						},
						Args: []ast.Expr{x.X},
					},
					Type: &ast.Ident{
						Name: vChild.info.TypeOf(x.X).String(),
					},
				}
				vChild.InsertStmtsAfter([]ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{
								Name: "gaptureGCtx.OnChanRecvDone",
							},
						},
					},
				})
			}

		case *ast.Ident:
			msg = fmt.Sprintf(" name: %s", x.Name)
		case *ast.BasicLit:
			msg = fmt.Sprintf(" value: %v", x.Value)
		case ast.Expr:
			t := vChild.info.TypeOf(x)
			if t != nil {
				msg = fmt.Sprintf(" type: %s", t.String())
			}
		}

		v.logf("%s %s%s%s", v.fileName, indent[0:depth],
			reflect.TypeOf(node).String(), msg)
	}

	return vChild
}

// MarkModified records that a converter (and its parents) have
// modified their associated ast.Node(s).
func (v *Converter) MarkModified() *Converter {
	if v != nil {
		v.modifications++
		v.parent.MarkModified() // Recursively mark our parents/ancestors.
	}

	return v
}

func (v *Converter) HasParentNode(node ast.Node) bool {
	for v != nil {
		if v.node == node {
			return true
		}
		v = v.parent
	}

	return false
}

// InsertStmtsAfter inserts the given stmt's after the stmt
// represented by the given converter node instance.
func (v *Converter) InsertStmtsAfter(toInsert []ast.Stmt) {
	var blockStmt *ast.BlockStmt
	for vc := v; vc != nil; vc = vc.parent { // Find the enclosing blockStmt.
		bs, ok := vc.node.(*ast.BlockStmt)
		if ok {
			blockStmt = bs
			break
		}
	}
	if blockStmt == nil {
		panic("AddCallStmtAfter could not find enclosing blockStmt")
	}

	idx := -1 // Find our position in the enclosing blockStmt.
	for i, stmt := range blockStmt.List {
		if v.HasParentNode(stmt) {
			idx = i
			break
		}
	}

	blockStmt.List = InsertStmts(blockStmt.List, idx+1, toInsert)
}

// InsertStmts inserts the given stmt's into a given position in a
// stmt list/array.
func InsertStmts(list []ast.Stmt, pos int, toInsert []ast.Stmt) []ast.Stmt {
	var rv []ast.Stmt
	rv = append(rv, list[0:pos]...)
	rv = append(rv, toInsert...)
	rv = append(rv, list[pos:]...)
	return rv
}
