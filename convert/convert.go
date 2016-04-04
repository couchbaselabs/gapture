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
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

var RuntimeVarName = "gaptureGCtx"
var RuntimePackageName = "github.com/couchbaselabs/gapture"

// RuntimeFuncPrefix is an AST snippet inserted as initialization
// stmt's in rewritten func bodies, in order to declare required vars.
var RuntimeFuncPrefix []ast.Stmt

func init() {
	expr, err := parser.ParseExpr("func() { var "+RuntimeVarName+" gapture.GCtx }")
	if err != nil {
		panic(err)
	}
	RuntimeFuncPrefix = expr.(*ast.FuncLit).Body.List
}

// ------------------------------------------------------

// Options allows users to override the default behavior of the
// instrumentation processing.
type Options struct {
	OnError func(error)
	Logf    func(fmt string, v ...interface{})
}

// ------------------------------------------------------

// ProcessDirs instruments the code in the given directory paths.
func ProcessProgram(prog *loader.Program, options Options) error {
	logf := options.Logf
	if logf == nil {
		logf = func(fmt string, v ...interface{}) { /* noop */ }
	}

	for pkg, pkgInfo := range prog.AllPackages {
		logf("pkg: %v => pkgInfo: %v", pkg, pkgInfo)

		for _, file := range pkgInfo.Files {
			converter := &Converter{
				info: &pkgInfo.Info,
				pkg:  pkg,
				file: file,
				logf: logf,
				node: file,
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

			err := format.Node(os.Stdout, prog.Fset, file)
			if err != nil {
				fmt.Println(err)
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
	parent *Converter
	info   *types.Info
	pkg    *types.Package
	file   *ast.File
	logf   func(fmt string, v ...interface{})
	node   ast.Node

	modifications int // Count of modifications made to this subtree.
}

var indent = "......................................................"

func (v *Converter) Visit(childNode ast.Node) ast.Visitor {
	vChild := &Converter{
		parent: v,
		info:   v.info,
		pkg:    v.pkg,
		file:   v.file,
		logf:   v.logf,
		node:   childNode,
	}

	if childNode != nil {
		depth := 0
		for vv := v; vv != nil; vv = vv.parent { // Indentation by depth.
			depth++
		}

		msg := ""

		switch x := childNode.(type) {
		case *ast.FuncDecl:
			msg = fmt.Sprintf(" name: %v", x.Name)
			if UsesChannels(v.info, x) {
				x.Body.List = InsertStmts(x.Body.List, 0, RuntimeFuncPrefix)
				vChild.MarkModified()
			}

		case *ast.FuncLit:
			if UsesChannels(v.info, x) {
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
								Name: RuntimeVarName + ".OnChanClose",
							},
							Args: x.Args,
						},
						Type: &ast.Ident{
							Name: types.TypeString(v.pkg, v.info.TypeOf(x.Args[0])),
						},
					},
				}
				vChild.InsertStmtsRelative(1, []ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{
								Name: RuntimeVarName + ".OnChanCloseDone",
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
			funName := RuntimeVarName + ".OnChanSend"
			var argsOp []ast.Expr

			commClause, commClausePos := v.PartOfSelectCommClause()
			if commClause != nil {
				funName = RuntimeVarName + ".OnChanSelectSend"
				posName := fmt.Sprintf("%d", commClausePos)
				argsOp = []ast.Expr{&ast.Ident{Name: posName}}

				commClause.Body = InsertStmts(commClause.Body, 0, []ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun:  &ast.Ident{Name: funName + "Done"},
							Args: []ast.Expr{&ast.Ident{Name: posName}},
						},
					},
				})
			} else {
				vChild.InsertStmtsRelative(1, []ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{Name: funName + "Done"},
						},
					},
				})
			}

			x.Chan = &ast.TypeAssertExpr{
				X: &ast.CallExpr{
					Fun:  &ast.Ident{Name: funName},
					Args: append(argsOp, x.Chan),
				},
				Type: &ast.Ident{
					Name: types.TypeString(v.pkg, v.info.TypeOf(x.Chan)),
				},
			}

			vChild.MarkModified()

		case *ast.UnaryExpr:
			// Convert:
			//   x, ok := <-chExpr
			// Into:
			//   x, ok := <-gaptureGCtx.OnChanRecv(chExpr).(chan foo))
			//   gaptureGCtx.OnChanRecvDone(nil)
			//
			// Convert:
			//   <-chExpr
			// Into:
			//   gaptureGCtx.OnChanRecvDone(
			//     <-gaptureGCtx.OnChanRecv(chExpr).(chan foo))).(foo)
			//
			if x.Op == token.ARROW {
				funName := RuntimeVarName + ".OnChanRecv"
				var argsOp []ast.Expr

				if _, ok := v.node.(*ast.AssignStmt); ok {
					commClause, commClausePos := v.PartOfSelectCommClause()
					if commClause != nil {
						funName = RuntimeVarName + ".OnChanSelectRecv"
						posName := fmt.Sprintf("%d", commClausePos)
						argsOp = []ast.Expr{&ast.Ident{Name: posName}}

						commClause.Body = InsertStmts(commClause.Body, 0, []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun:  &ast.Ident{Name: funName + "Done"},
									Args: []ast.Expr{
										&ast.Ident{Name: posName},
									},
								},
							},
						})
					} else {
						vChild.InsertStmtsRelative(1, []ast.Stmt{
							&ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.Ident{Name: funName + "Done"},
									Args: []ast.Expr{
										&ast.Ident{Name: "nil"},
									},
								},
							},
						})
					}

					x.X = &ast.TypeAssertExpr{
						X: &ast.CallExpr{
							Fun:  &ast.Ident{Name: funName},
							Args: append(argsOp, x.X),
						},
						Type: &ast.Ident{
							Name: types.TypeString(v.pkg, v.info.TypeOf(x.X)),
						},
					}

					vChild.MarkModified()
				} else {
					ast.Walk(&Converter{
						info: vChild.info,
						pkg:  vChild.pkg,
						file: vChild.file,
						logf: vChild.logf,
						node: x.X,
					}, x.X)

					typeUnderlying := v.info.TypeOf(x.X).Underlying()

					x.X = &ast.TypeAssertExpr{
						X: &ast.CallExpr{
							Fun:  &ast.Ident{Name: funName},
							Args: append(argsOp, x.X),
						},
						Type: &ast.Ident{
							Name: types.TypeString(v.pkg, v.info.TypeOf(x.X)),
						},
					}

					childNode = v.ReplaceChildExpr(x,
						&ast.TypeAssertExpr{
							X: &ast.CallExpr{
								Fun:  &ast.Ident{Name: RuntimeVarName + ".OnChanRecvDone"},
								Args: []ast.Expr{x},
							},
							Type: &ast.Ident{
								Name: types.TypeString(v.pkg, typeUnderlying),
							},
						})

					vChild.node = childNode

					vChild.MarkModified()

					return nil
				}
			}

		case *ast.SelectStmt:
			// Convert:
			//   select {
			//   case msg := <-recvCh:
			//   case sendCh <- msgExpr:
			//   default:
			//   }
			// Into:
			//   select {
			//   case msg := <-gaptureGCtx.OnChanSelectRecv(0, recvCh).(chan foo):
			//     gaptureGCtx.OnChanSelectRecvDone(0)
			//   case gaptureGCtx.OnChanSelectSend(1, chExpr).(chan foo) <- msgExpr:
			//     gaptureGCtx.OnChanSelectSendDone(1)
			//   default:
			//     gaptureGCtx.OnChanSelectDefault()
			//   }
			//
			if x.Body != nil {
				for _, stmt := range x.Body.List {
					commClause, ok := stmt.(*ast.CommClause)
					if ok && commClause.Comm == nil { // The 'default:' case.
						commClause.Body = InsertStmts(commClause.Body, 0,
							[]ast.Stmt{
								&ast.ExprStmt{
									X: &ast.CallExpr{
										Fun: &ast.Ident{
											Name: RuntimeVarName + ".OnChanSelectDefault",
										},
									},
								},
							})
						vChild.MarkModified()
					}
				}
			}

		case *ast.RangeStmt:
			// Convert:
			//   for msg := range chExpr { ... }
			// Info:
			//   for msg := range gaptureGCtx.OnChanRange(chExpr).(chan foo) {
			//     gaptureGCtx.OnChanRangeBody()
			//     ...
			//     ISSUE: any continue's here skip the OnChanRangeBodyLoop!!!
			//     ...
			//     gaptureGCtx.OnChanRangeBodyContinue()
			//   }
			//   gaptureGCtx.OnChanRangeDone()
			//
			xType := v.info.TypeOf(x.X)
			xTypeString := types.TypeString(v.pkg, xType)
			if strings.HasPrefix(xTypeString, "chan ") {
				x.X = &ast.TypeAssertExpr{
					X: &ast.CallExpr{
						Fun:  &ast.Ident{Name: RuntimeVarName + ".OnChanRange"},
						Args: []ast.Expr{x.X},
					},
					Type: &ast.Ident{Name: xTypeString},
				}

				x.Body.List = InsertStmts(x.Body.List, 0, []ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{Name: RuntimeVarName + ".OnChanRangeBody"},
						},
					},
				})

				x.Body.List = append(x.Body.List,
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{Name: RuntimeVarName + ".OnChanRangeBodyContinue"},
						},
					})

				vChild.InsertStmtsRelative(1, []ast.Stmt{
					&ast.ExprStmt{
						X: &ast.CallExpr{
							Fun: &ast.Ident{Name: RuntimeVarName + ".OnChanRangeDone"},
						},
					},
				})

				vChild.MarkModified()
			}

		case *ast.Ident:
			msg = fmt.Sprintf(" name: %s", x.Name)
		case *ast.BasicLit:
			msg = fmt.Sprintf(" value: %v", x.Value)
		case ast.Expr:
			t := v.info.TypeOf(x)
			if t != nil {
				msg = fmt.Sprintf(" type: %s", types.TypeString(v.pkg, t))
			}
		}

		v.logf("%s%s%s", indent[0:depth], reflect.TypeOf(childNode).String(), msg)
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

// ReplaceChildExpr replaces a direct child orig Expr with a
// replacement Expr, and returns the replacement Expr (as an
// ast.Node).
func (v *Converter) ReplaceChildExpr(orig, replacement ast.Expr) ast.Node {
	replaceInExprList := func(exprs []ast.Expr) {
		for i, expr := range exprs {
			if expr == orig {
				exprs[i] = replacement
			}
		}
	}

	switch n := v.node.(type) {
	// Expressions
	case *ast.BadExpr, *ast.Ident, *ast.BasicLit:
		// NO-OP.

	case *ast.Ellipsis:
		if n.Elt == orig {
			n.Elt = replacement
		}

	case *ast.FuncLit:
		// NO-OP.

	case *ast.CompositeLit:
		replaceInExprList(n.Elts)

	case *ast.ParenExpr:
		if n.X == orig {
			n.X = replacement
		}

	case *ast.SelectorExpr:
		if n.X == orig {
			n.X = replacement
		}

	case *ast.IndexExpr:
		if n.X == orig {
			n.X = replacement
		}
		if n.Index == orig {
			n.Index = replacement
		}

	case *ast.SliceExpr:
		if n.Low == orig {
			n.Low = replacement
		}
		if n.High == orig {
			n.High = replacement
		}
		if n.Max == orig {
			n.Max = replacement
		}

	case *ast.TypeAssertExpr:
		if n.X == orig {
			n.X = replacement
		}
		if n.Type == orig {
			n.Type = replacement
		}

	case *ast.CallExpr:
		if n.Fun == orig {
			n.Fun = replacement
		}
		replaceInExprList(n.Args)

	case *ast.StarExpr:
		if n.X == orig {
			n.X = replacement
		}

	case *ast.UnaryExpr:
		if n.X == orig {
			n.X = replacement
		}

	case *ast.BinaryExpr:
		if n.X == orig {
			n.X = replacement
		}
		if n.Y == orig {
			n.Y = replacement
		}

	case *ast.KeyValueExpr:
		if n.Key == orig {
			n.Key = replacement
		}
		if n.Value == orig {
			n.Value = replacement
		}

	// Types
	case *ast.ArrayType:
		// NO-OP.

	case *ast.StructType:
		// NO-OP.

	case *ast.FuncType:
		// NO-OP.

	case *ast.InterfaceType:
		// NO-OP.

	case *ast.MapType:
		// NO-OP.

	case *ast.ChanType:
		// NO-OP.

	// Statements
	case *ast.BadStmt:
		// NO-OP.

	case *ast.DeclStmt:
		// NO-OP.

	case *ast.EmptyStmt:
		// NO-OP.

	case *ast.LabeledStmt:
		// NO-OP.

	case *ast.ExprStmt:
		if n.X == orig {
			n.X = replacement
		}

	case *ast.SendStmt:
		if n.Chan == orig {
			n.Chan = replacement
		}
		if n.Value == orig {
			n.Value = replacement
		}

	case *ast.IncDecStmt:
		if n.X == orig {
			n.X = replacement
		}

	case *ast.AssignStmt:
		replaceInExprList(n.Lhs)
		replaceInExprList(n.Rhs)

	case *ast.GoStmt:
		// NO-OP.

	case *ast.DeferStmt:
		// NO-OP.

	case *ast.ReturnStmt:
		replaceInExprList(n.Results)

	case *ast.BranchStmt:
		// NO-OP.

	case *ast.BlockStmt:
		// NO-OP.

	case *ast.IfStmt:
		if n.Cond == orig {
			n.Cond = replacement
		}

	case *ast.CaseClause:
		replaceInExprList(n.List)

	case *ast.SwitchStmt:
		if n.Tag == orig {
			n.Tag = replacement
		}

	case *ast.TypeSwitchStmt:
		// NO-OP.

	case *ast.CommClause:
		// NO-OP.

	case *ast.SelectStmt:
		// NO-OP.

	case *ast.ForStmt:
		if n.Cond == orig {
			n.Cond = replacement
		}

	case *ast.RangeStmt:
		if n.Key == orig {
			n.Key = replacement
		}
		if n.Value == orig {
			n.Value = replacement
		}
		if n.X == orig {
			n.X = replacement
		}

	// Declarations
	case *ast.ImportSpec:
		// NO-OP.

	case *ast.ValueSpec:
		replaceInExprList(n.Values)

	case *ast.TypeSpec:
		// NO-OP.

	case *ast.BadDecl:
		// NO-OP.

	case *ast.GenDecl:
		// NO-OP.

	case *ast.FuncDecl:
		// NO-OP.

	// Files and packages
	case *ast.File:
		// NO-OP.

	case *ast.Package:
		// NO-OP.

	default:
		// NO-OP.
	}

	return replacement
}

// PartOfSelectCommClause returns the 0-based position of the
// case/default clause if the node is part of a select case/default
// CommClause.
func (v *Converter) PartOfSelectCommClause() (*ast.CommClause, int) {
	for v != nil {
		commClause, ok := v.node.(*ast.CommClause)
		if ok {
			blockStmt, ok := v.parent.node.(*ast.BlockStmt)
			if !ok {
				panic("PartOfSelectCommClause expected a BlockStmt")
			}

			for i, cc := range blockStmt.List {
				if cc == commClause {
					return commClause, i
				}
			}

			panic("PartOfSelectCommClause expected CommClause to be found")
		}

		// If we see a Stmt while walking up our parent/ancestry, and
		// it's not an AssignStmt (`x := <-ch`), then we're not in a
		// select CommClause.
		_, ok = v.node.(ast.Stmt)
		if ok {
			_, ok = v.node.(*ast.AssignStmt)
			if !ok {
				return nil, -1
			}
		}

		v = v.parent
	}

	return nil, -1
}

// InsertStmtsRelative inserts the given stmt's after the stmt
// represented by the given converter node instance.  Use posDelta of
// 1 to insert after; and posDelta of 0 to insert before.
func (v *Converter) InsertStmtsRelative(posDelta int, toInsert []ast.Stmt) {
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

	blockStmt.List = InsertStmts(blockStmt.List, idx+posDelta, toInsert)
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
