package paragraph_test

import (
	"bytes"
	"go/ast"
	"go/format"
	"testing"
)

// Audit the complete equal-width branch, not just the fitter it calls. Only the
// exact width-change branch may enter reflow; every other resolved helper is
// checked transitively with P1's service/breaker whitelist.
func TestFlowReuseCallBoundary(t *testing.T) {
	w := newAuditWorld(t)
	p, err := w.load(paragraphPath)
	if err != nil {
		t.Fatal(err)
	}
	fn := p.functions["(*"+paragraphPath+".Continuation).Fit"]
	if fn == nil {
		t.Fatal("missing dispatcher")
	}
	body := &ast.BlockStmt{}
	skipped := 0
	for _, stmt := range fn.Body.List {
		if branch, ok := stmt.(*ast.IfStmt); ok {
			var condition bytes.Buffer
			if err := format.Node(&condition, w.fset, branch.Cond); err != nil {
				t.Fatal(err)
			}
			if condition.String() == "frame.Width != c.width && c.remainder.Range().Start != c.remainder.Range().End" {
				skipped++
				continue
			}
		}
		body.List = append(body.List, stmt)
	}
	if skipped != 1 {
		t.Fatal("review changed dispatch guard", skipped)
	}
	a := &callAudit{world: w, seen: make(map[string]bool)}
	a.inspect(p, body)
	if a.err != nil || len(a.issues) != 0 {
		t.Fatal(a.issues, a.err)
	}
}
