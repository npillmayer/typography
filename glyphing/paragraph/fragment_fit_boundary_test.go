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
