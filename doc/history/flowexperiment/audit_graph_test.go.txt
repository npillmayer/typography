package flowexperiment

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
)

const paragraphPath = "github.com/npillmayer/typography/glyphing/paragraph"
const layoutPath = "github.com/npillmayer/typography/layout"
const khipuPath = "github.com/npillmayer/khipu"

// The only opaque leaves are standard pure operations and context.Err.
// Project methods below are identified by declaring package and receiver, then
// their bodies are inspected recursively, including private helpers in any file.
var auditLeaves = map[string]bool{
	"errors.New": true, "fmt.Errorf": true, "math.Abs": true, "math.IsNaN": true,
	"math.IsInf": true, "math.Max": true, "slices.Clone": true, "(context.Context).Err": true,
}

var auditEntries = map[string]bool{
	paragraphPath + ".Fragment": true, paragraphPath + ".SplitFragment": true,
	layoutPath + ".NewFragment":          true,
	"(*" + layoutPath + ".VList).Frozen": true, "(*" + layoutPath + ".VList).Lines": true,
	"(*" + layoutPath + ".VList).Materials": true, "(*" + layoutPath + ".VList).AtomicPlacements": true, "(*" + layoutPath + ".VList).Box": true,
	"(*" + layoutPath + ".Fragment).Frozen": true, "(*" + layoutPath + ".Fragment).Lines": true,
	"(*" + layoutPath + ".Fragment).Materials": true, "(*" + layoutPath + ".Fragment).AtomicPlacements": true,
	"(*" + layoutPath + ".Fragment).Box": true, "(*" + layoutPath + ".Fragment).Range": true,
	"(" + layoutPath + ".Box).Depth":             true,
	"(*" + khipuPath + ".FrozenKhipu).LineCount": true, "(*" + khipuPath + ".FrozenKhipu).Line": true,
	"(*" + khipuPath + ".FrozenKhipu).Atomic": true, "(*" + khipuPath + ".FrozenKhipu).KnotCount": true,
	"(" + khipuPath + "/dimen.DU).Points": true,
}

type callAudit struct {
	world  *auditWorld
	seen   map[string]bool
	issues []string
	err    error
}

func auditGeometry(w *auditWorld) ([]string, error) {
	a := &callAudit{world: w, seen: make(map[string]bool)}
	p, err := w.load(experimentPath)
	if err != nil {
		return nil, err
	}
	imports := map[string]bool{"context": true, "errors": true, "fmt": true, "math": true,
		khipuPath + "/contracts": true, khipuPath + "/dimen": true, layoutPath: true, paragraphPath: true}
	// Inspect every E1 implementation file/body/initializer, even unused helpers.
	for _, f := range p.files {
		for _, spec := range f.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if !imports[path] || spec.Name != nil && (spec.Name.Name == "." || spec.Name.Name == "_") {
				a.issues = append(a.issues, "forbidden E1 import "+path)
			}
		}
		a.inspect(p, f)
	}
	slices.Sort(a.issues)
	return slices.Compact(a.issues), a.err
}

func (a *callAudit) function(fn *types.Func) {
	name := fn.Origin().FullName()
	if a.seen[name] || auditLeaves[name] {
		return
	}
	a.seen[name] = true
	path := fn.Pkg().Path()
	private := !fn.Exported() && (path == paragraphPath || path == layoutPath || path == khipuPath)
	if path != experimentPath && !private && !auditEntries[name] {
		a.issues = append(a.issues, "forbidden function "+name)
		return
	}
	p, err := a.world.load(path)
	if err != nil {
		a.err = err
		return
	}
	decl := p.functions[name]
	if decl == nil || decl.Body == nil {
		a.err = fmt.Errorf("missing audited body for %s", name)
		return
	}
	a.inspect(p, decl.Body)
}

func (a *callAudit) inspect(p *auditPackage, node ast.Node) {
	closures := literalClosures(p, node)
	ast.Inspect(node, func(n ast.Node) bool {
		// Inspect function references, not only direct calls: this also catches
		// aliases and method values assigned to variables before invocation.
		if id, ok := n.(*ast.Ident); ok {
			if fn, ok := p.info.Uses[id].(*types.Func); ok {
				a.function(fn)
			}
		}
		if call, ok := n.(*ast.CallExpr); ok && !p.info.Types[call.Fun].IsType() {
			fun := unindex(call.Fun)
			var obj types.Object
			switch f := fun.(type) {
			case *ast.Ident:
				obj = p.info.Uses[f]
			case *ast.SelectorExpr:
				obj = p.info.Uses[f.Sel]
			case *ast.FuncLit:
				return true // Its body is visited normally.
			}
			switch obj.(type) {
			case *types.Func, *types.Builtin: // Resolved references checked above.
			default:
				if !closures[obj] {
					a.issues = append(a.issues, "unresolved callable at "+a.world.fset.Position(call.Pos()).String())
				}
			}
		}
		return a.err == nil
	})
}

func unindex(e ast.Expr) ast.Expr {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return unindex(e.X)
	case *ast.IndexExpr:
		return unindex(e.X)
	case *ast.IndexListExpr:
		return unindex(e.X)
	default:
		return e
	}
}

// A local, single-assignment literal (NewFragment's bad closure) is inspectable.
// Reject callback parameters, reassigned function variables and escaped pointers
// instead of treating an unresolved call as evidence of zero services.
func literalClosures(p *auditPackage, node ast.Node) map[types.Object]bool {
	allowed := make(map[types.Object]bool)
	writes := make(map[types.Object]int)
	ast.Inspect(node, func(n ast.Node) bool {
		if s, ok := n.(*ast.AssignStmt); ok {
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					obj := p.info.ObjectOf(id)
					writes[obj]++
					if i < len(s.Rhs) && s.Tok == token.DEFINE {
						_, allowed[obj] = s.Rhs[i].(*ast.FuncLit)
					}
				}
			}
		}
		if u, ok := n.(*ast.UnaryExpr); ok && u.Op == token.AND {
			if id, ok := unindex(u.X).(*ast.Ident); ok {
				writes[p.info.ObjectOf(id)] += 2
			}
		}
		return true
	})
	for obj := range allowed {
		allowed[obj] = allowed[obj] && writes[obj] == 1
	}
	return allowed
}
