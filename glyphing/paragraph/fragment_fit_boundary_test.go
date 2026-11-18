package paragraph_test

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFitGeometryOnlyCallBoundary(t *testing.T) {
	w := newAuditWorld(t)
	issues, err := auditGeometry(w)
	if err != nil || len(issues) != 0 {
		t.Fatal(issues, err)
	}
	data, err := os.ReadFile(filepath.Join(w.listings[paragraphPath].Dir, "fragment_fit.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, body, want string }{
		{"breaker in another file", `import ("github.com/npillmayer/khipu"; "github.com/npillmayer/khipu/linebreak/firstfit")
func fitExtra() { _, _ = firstfit.BreakParagraph(nil, khipu.Params{}, nil) }`, "forbidden function " + khipuPath + "/linebreak/firstfit.BreakParagraph"},
		{"method value", `func fitExtra() { var p *Prepared; f := p.Layout; _ = f }`, "forbidden function (*" + paragraphPath + ".Prepared).Layout"},
		{"function alias", `func fitExtra() { f := Run; _ = f }`, "forbidden function " + paragraphPath + ".Run"},
		{"unknown callback", `var fitCallback func(); func fitExtra() { fitCallback() }`, "unresolved callable"},
		{"pure helper", `import "math"; func fitExtra() { _ = math.Abs(-1) }`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := *w
			m.packages = maps.Clone(w.packages)
			delete(m.packages, paragraphPath)
			m.mutations = map[string]map[string]string{paragraphPath: {
				"fragment_fit.go": strings.Replace(string(data), "out := new(FragmentFit)", "fitExtra()\n out := new(FragmentFit)", 1),
				"fit_extra.go":    "package paragraph\n" + tc.body,
			}}
			issues, err := auditGeometry(&m)
			if err != nil {
				t.Fatal("invalid audit mutation:", err)
			}
			if tc.want == "" {
				if len(issues) != 0 {
					t.Fatal(issues)
				}
			} else if !strings.Contains(strings.Join(issues, "\n"), tc.want) {
				t.Fatal("missed forbidden call", tc.want, issues)
			}
		})
	}
}

// Preserve the experiment's transitive audit regression: a geometry entry must
// not hide a breaker call in a delegated helper in another source file.
func TestFragmentGeometryDelegatedCallBoundary(t *testing.T) {
	w := newAuditWorld(t)
	data, err := os.ReadFile(filepath.Join(w.listings[paragraphPath].Dir, "fragment.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ entry, anchor string }{
		{"Fragment", "lines := v.Lines()"},
		{"FitFragment", "origin := lines[0].Top"},
	} {
		t.Run(tc.entry, func(t *testing.T) {
			if strings.Count(string(data), tc.anchor) != 1 {
				t.Fatal("review changed mutation anchor", tc.anchor)
			}
			for _, mutated := range []bool{false, true} {
				m := *w
				m.packages = maps.Clone(w.packages)
				delete(m.packages, paragraphPath)
				if mutated {
					m.mutations = map[string]map[string]string{paragraphPath: {
						"fragment.go": strings.Replace(string(data), tc.anchor, "extraBreak()\n"+tc.anchor, 1),
						"extra.go": `package paragraph
import ("github.com/npillmayer/khipu"; "github.com/npillmayer/khipu/linebreak/firstfit")
func extraBreak() { _, _ = firstfit.BreakParagraph(nil, khipu.Params{}, nil) }`,
					}}
				}
				p, err := m.load(paragraphPath)
				if err != nil {
					t.Fatal("invalid audit source:", err)
				}
				fn := p.functions[paragraphPath+"."+tc.entry]
				if fn == nil || fn.Body == nil {
					t.Fatal("missing audit entry", tc.entry)
				}
				a := &callAudit{world: &m, seen: make(map[string]bool)}
				a.inspect(p, fn.Body)
				if a.err != nil {
					t.Fatal(a.err)
				}
				want := "forbidden function " + khipuPath + "/linebreak/firstfit.BreakParagraph"
				if mutated && !strings.Contains(strings.Join(a.issues, "\n"), want) {
					t.Fatal("missed delegated breaker call", a.issues)
				}
				if !mutated && len(a.issues) != 0 {
					t.Fatal(a.issues)
				}
			}
		})
	}
}
