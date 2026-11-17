package integration_test

import (
	"testing"

	glyphot "github.com/npillmayer/typography/glyphing/adapter/opentype"
)

func fixture(t *testing.T) *glyphot.Face {
	t.Helper()
	f, err := glyphot.LoadFile("../../opentype/testdata/fonts/ShaperTestProgress.ttf")
	if err != nil {
		t.Fatal(err)
	}
	return f
}
