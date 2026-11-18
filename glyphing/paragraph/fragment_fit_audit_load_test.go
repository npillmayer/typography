package paragraph_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const fitAuditPath = "github.com/npillmayer/typography/glyphing/paragraph"

type auditListing struct {
	ImportPath, Dir, Export string
	GoFiles, IgnoredGoFiles []string
}

type auditPackage struct {
	files     []*ast.File
	info      *types.Info
	functions map[string]*ast.FuncDecl
}

// Use module-aware export data with the standard type checker, without adding
// a production dependency. Source bodies are loaded separately for inspection.
type auditWorld struct {
	fset     *token.FileSet
	importer types.Importer
	listings map[string]auditListing
	packages map[string]*auditPackage
	// Mutations are in-memory source replacements, including newly added files.
	mutations map[string]map[string]string
}

func newAuditWorld(t *testing.T) *auditWorld {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-export", "-json", ".")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		t.Fatalf("load audit dependencies: %v: %s", err, stderr.String())
	}
	w := &auditWorld{fset: token.NewFileSet(), listings: make(map[string]auditListing), packages: make(map[string]*auditPackage)}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var p auditListing
		if err := decoder.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		w.listings[p.ImportPath] = p
	}
	w.importer = importer.ForCompiler(w.fset, "gc", func(path string) (io.ReadCloser, error) {
		p, ok := w.listings[path]
		if !ok || p.Export == "" {
			return nil, fmt.Errorf("missing export data for %s", path)
		}
		return os.Open(p.Export)
	})
	return w
}

func (w *auditWorld) load(path string) (*auditPackage, error) {
	if p := w.packages[path]; p != nil {
		return p, nil
	}
	l, ok := w.listings[path]
	if !ok {
		return nil, fmt.Errorf("missing source package %s", path)
	}
	if path == fitAuditPath && len(l.IgnoredGoFiles) != 0 {
		return nil, fmt.Errorf("Paragraph build-specific source requires audit policy: %v", l.IgnoredGoFiles)
	}
	names := slices.Clone(l.GoFiles)
	for name := range w.mutations[path] {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	p := &auditPackage{info: &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object), Defs: make(map[*ast.Ident]types.Object)}, functions: make(map[string]*ast.FuncDecl)}
	for _, name := range names {
		var source any
		if s, ok := w.mutations[path][name]; ok {
			source = s
		}
		f, err := parser.ParseFile(w.fset, filepath.Join(l.Dir, name), source, 0)
		if err != nil {
			return nil, err
		}
		p.files = append(p.files, f)
	}
	config := types.Config{Importer: w.importer}
	if _, err := config.Check(path, w.fset, p.files, p.info); err != nil {
		return nil, err
	}
	for _, f := range p.files {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				p.functions[p.info.Defs[fn.Name].(*types.Func).FullName()] = fn
			}
		}
	}
	w.packages[path] = p
	return p, nil
}
