package font_test

import (
	"bytes"
	"testing"

	"github.com/npillmayer/khipu/contracts"
	"github.com/npillmayer/khipu/dimen"
	fontmeta "github.com/npillmayer/typography/font"
	"github.com/npillmayer/typography/fontresource"
)

type face struct {
	r    *fontresource.Resource
	info fontmeta.Info
}

func (*face) Metrics() (contracts.FontMetrics, error)     { panic("identity queried metrics") }
func (*face) Scale(dimen.DU) (contracts.FontScale, error) { panic("identity queried scale") }
func (f *face) Resource() *fontresource.Resource          { return f.r }
func (f *face) Info() fontmeta.Info                       { return f.info }
func TestRetainedIdentity(t *testing.T) {
	r, err := fontresource.Read(bytes.NewBufferString("retained font bytes"), fontresource.Origin{Location: "missing.ttf"})
	if err != nil {
		t.Fatal(err)
	}
	f := &face{r, fontmeta.Info{ResourceID: r.ID(), SHA256: r.SHA256(), Origin: r.Origin(), Path: r.Origin().Location, Warnings: []string{"warning"}}}
	i, err := fontmeta.Identify(f)
	if err != nil {
		t.Fatal(err)
	}
	i.Warnings[0] = "changed"
	if f.info.Warnings[0] != "warning" {
		t.Fatal("identity aliases warning slice")
	}
	for _, v := range []contracts.FontFace{nil, (*face)(nil), &face{}} {
		if _, err := fontmeta.Identify(v); err == nil {
			t.Fatal("invalid face accepted")
		}
	}
	for _, mutate := range []func(*fontmeta.Info){
		func(i *fontmeta.Info) { i.ResourceID = "other" }, func(i *fontmeta.Info) { i.SHA256 = "other" },
		func(i *fontmeta.Info) { i.Path = "other" }, func(i *fontmeta.Info) { i.Origin.Name = "other" },
	} {
		bad := *f
		mutate(&bad.info)
		if _, err := fontmeta.Identify(&bad); err == nil {
			t.Fatal("mismatched identity accepted")
		}
	}
}
