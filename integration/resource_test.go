package integration_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/npillmayer/typography/fontresource"
	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
	renderot "github.com/npillmayer/typography/render/adapter/opentype"
)

func TestResourceRetentionAndOutlineConcurrency(t *testing.T) {
	data, err := os.ReadFile("../../opentype/testdata/fonts/GentiumPlus-R.ttf")
	if err != nil {
		t.Fatal(err)
	}
	r, err := fontresource.Read(bytes.NewReader(data), fontresource.Origin{Provider: "contract", Name: "Gentium", Location: "font.ttf"})
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	f, err := glyphot.LoadResource(r)
	if err != nil {
		t.Fatal(err)
	}
	if f.Info().SHA256 != "fa09788adcc651d77eba7d96f8cc3a600d6484c58e3b4ff79c2b2464dbfea6f8" || f.Info().ResourceID != "sha256:cdc0fada101940c0545e4670c40bf9e5d8d9fbf91f50e9f3a432e0b557de5a58" {
		t.Fatalf("identity changed %+v", f.Info())
	}
	retained, err := io.ReadAll(f.Resource().Open())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err = os.WriteFile(path, retained, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := glyphot.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	gid, err := file.GlyphIndex('a')
	if err != nil || gid == 0 {
		t.Fatal("coverage", err)
	}
	renderer := renderot.New()
	want, err := renderer.Outline(file, gid)
	if err != nil || len(want.Points) == 0 {
		t.Fatal("retained outline", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g, e := file.GlyphIndex('a')
			if e != nil || g != gid {
				t.Error("concurrent coverage", e)
			}
			outline, e := renderer.Outline(file, gid)
			if e != nil || !reflect.DeepEqual(outline, want) {
				t.Error("concurrent outline", e)
			}
			if len(outline.Points) > 0 {
				outline.Points[0].X++
			}
		}()
	}
	wg.Wait()
	if _, err := glyphot.LoadResource(nil); err == nil {
		t.Fatal("nil resource")
	}
	bad, _ := fontresource.Read(bytes.NewReader([]byte("bad font")), fontresource.Origin{})
	if _, err := glyphot.LoadResource(bad); err == nil {
		t.Fatal("suppressed parser failure")
	}
}
