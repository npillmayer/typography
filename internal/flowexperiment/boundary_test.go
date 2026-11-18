package flowexperiment

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Backend traps run after layout; this structural audit covers the operations
// without injectable counters, including calls hidden in additional source files.
func TestE1GeometryOnlyCallBoundary(t *testing.T) {
	w := newAuditWorld(t)
	issues, err := auditGeometry(w)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatal(strings.Join(issues, "\n"))
	}

	// Mutation regressions use the same loader/checker on edited source in memory.
	// No fixture or repository file is changed; a type error cannot count as a
	// successful detection of a forbidden call.
	read := func(pkg, name string) string {
		b, err := os.ReadFile(filepath.Join(w.listings[pkg].Dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	split := read(experimentPath, "split.go")
	fragment := read(paragraphPath, "fragment.go")
	for _, tc := range []struct {
		name, pkg, want string
		sources         map[string]string
	}{
		{"new file breaker helper", experimentPath, "forbidden function " + khipuPath + "/linebreak/firstfit.BreakParagraph", map[string]string{
			"split.go": strings.Replace(split, "whole, err := paragraph.Fragment", "extraBreak()\n whole, err := paragraph.Fragment", 1),
			"extra.go": `package flowexperiment
import ("github.com/npillmayer/khipu"; "github.com/npillmayer/khipu/linebreak/firstfit")
func extraBreak() { _, _ = firstfit.BreakParagraph(nil, khipu.Params{}, nil) }`,
		}},
		{"function alias", experimentPath, "forbidden function " + paragraphPath + ".Run", map[string]string{
			"extra.go": `package flowexperiment
import p "github.com/npillmayer/typography/glyphing/paragraph"
var runAgain = p.Run`,
		}},
		{"method value", experimentPath, "forbidden function (*" + paragraphPath + ".Prepared).Layout", map[string]string{
			"extra.go": `package flowexperiment
import "github.com/npillmayer/typography/glyphing/paragraph"
func extra(p *paragraph.Prepared) { f := p.Layout; _ = f }`,
		}},
		{"unknown callback", experimentPath, "unresolved callable", map[string]string{
			"extra.go": `package flowexperiment
func extra(f func()) { f() }`,
		}},
		{"delegated helper in another file", paragraphPath, "forbidden function " + khipuPath + "/linebreak/firstfit.BreakParagraph", map[string]string{
			"fragment.go": strings.Replace(fragment, "lines := v.Lines()", "extraBreak()\n lines := v.Lines()", 1),
			"extra.go": `package paragraph
import ("github.com/npillmayer/khipu"; "github.com/npillmayer/khipu/linebreak/firstfit")
func extraBreak() { _, _ = firstfit.BreakParagraph(nil, khipu.Params{}, nil) }`,
		}},
		{"pure helper in another file", experimentPath, "", map[string]string{
			"split.go": strings.Replace(split, "math.Abs(v)", "magnitude(v)", 1),
			"extra.go": `package flowexperiment
import m "math"
func magnitude(v float64) float64 { return m.Abs(v) }`,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mutated := *w
			mutated.packages = maps.Clone(w.packages)
			delete(mutated.packages, tc.pkg)
			mutated.mutations = map[string]map[string]string{tc.pkg: tc.sources}
			issues, err := auditGeometry(&mutated)
			if err != nil {
				t.Fatal("invalid mutation fixture:", err)
			}
			if tc.want == "" {
				if len(issues) != 0 {
					t.Fatal(issues)
				}
			} else if !strings.Contains(strings.Join(issues, "\n"), tc.want) {
				t.Fatalf("audit missed mutation: want %s, got %v", tc.want, issues)
			}
		})
	}
}
