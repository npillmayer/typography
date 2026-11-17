package opentype

import (
	"bytes"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
)

// A rendering-only face deliberately cannot supply metrics/scaling or shaping.
// It shares no concrete type or parser with the glyphing adapter.
type retainedFace struct {
	resource *fontresource.Resource
	info     fontmeta.Info
}

func (*retainedFace) Metrics() (contracts.FontMetrics, error)     { panic("render queried metrics") }
func (*retainedFace) Scale(dimen.DU) (contracts.FontScale, error) { panic("render queried scale") }
func (f *retainedFace) Resource() *fontresource.Resource          { return f.resource }
func (f *retainedFace) Info() fontmeta.Info                       { return f.info }
func fixture(t *testing.T) *retainedFace {
	t.Helper()
	data, err := os.ReadFile("../../../../opentype/testdata/fonts/ShaperTestProgress.ttf")
	if err != nil {
		t.Fatal(err)
	}
	r, err := fontresource.Read(bytes.NewReader(data), fontresource.Origin{Location: "nonexistent-original.ttf"})
	if err != nil {
		t.Fatal(err)
	}
	return &retainedFace{r, fontmeta.Info{ResourceID: r.ID(), SHA256: r.SHA256(), Origin: r.Origin(), Path: r.Origin().Location, Warnings: []string{"retained"}}}
}
func TestRenderingWithoutGlyphingServices(t *testing.T) {
	f := fixture(t)
	a, b := New(), New()
	want, err := a.Outline(f, 1)
	if err != nil || want.Path == "" {
		t.Fatal(err)
	}
	got, err := b.Outline(f, 1)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("independent services differ", err)
	}
	if len(a.fonts) != 1 || len(b.fonts) != 1 {
		t.Fatal("missing session cache")
	}
	if len(a.identities) != 1 || len(b.identities) != 1 {
		t.Fatal("missing retained-resource validation cache")
	}
	info, err := a.Identify(f)
	if err != nil {
		t.Fatal(err)
	}
	info.Warnings[0] = "changed"
	if f.info.Warnings[0] != "retained" {
		t.Fatal("shared mutable identity")
	}
	other := *f
	other.info.Family = "another face label"
	if info, err := a.Identify(&other); err != nil || info.Family != other.info.Family {
		t.Fatal("resource cache replaced face metadata", err)
	}
	if _, err := (*Services)(nil).Identify(f); err == nil {
		t.Fatal("nil identity service accepted")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 8 {
				got, err := a.Outline(f, 1)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Error("concurrent outline", err)
				}
				if len(got.Points) > 0 {
					got.Points[0].X++
				}
			}
		})
	}
	wg.Wait()
	for _, face := range []contracts.FontFace{nil, (*retainedFace)(nil), &retainedFace{}} {
		if _, err := a.Outline(face, 1); err == nil {
			t.Fatal("accepted invalid face")
		}
	}
	bad := *f
	bad.info.SHA256 = "wrong"
	if _, err := a.Outline(&bad, 1); err == nil {
		t.Fatal("accepted mismatched resource")
	}
	if _, err := a.Outline(f, 65536); err == nil {
		t.Fatal("accepted invalid glyph")
	}
	// Failed parsing must not poison the session.
	r, _ := fontresource.Read(bytes.NewBufferString("bad font"), fontresource.Origin{})
	broken := &retainedFace{r, fontmeta.Info{ResourceID: r.ID(), SHA256: r.SHA256(), Origin: r.Origin()}}
	if _, err := a.Outline(broken, 1); err == nil || len(a.fonts) != 1 {
		t.Fatal("cached failed parse")
	}
	if got, err := a.Outline(f, 1); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("failure damaged cache", err)
	}
}
