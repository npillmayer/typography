package typography_test

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func dependencies(t *testing.T, roots ...string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	args := append([]string{"list", "-deps", "-json"}, roots...)
	data, err := exec.CommandContext(ctx, "go", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("dependency inspection: %v: %s", err, data)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	var paths []string
	for {
		var pkg struct{ ImportPath string }
		err = dec.Decode(&pkg)
		if err == io.EOF {
			return paths
		}
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, pkg.ImportPath)
	}
}
func forbid(t *testing.T, paths []string, blocked ...string) {
	t.Helper()
	for _, p := range paths {
		for _, b := range blocked {
			if p == b || strings.HasPrefix(p, b+"/") {
				t.Fatalf("forbidden dependency %s", p)
			}
		}
	}
}
func TestPackageArms(t *testing.T) {
	const root = "github.com/npillmayer/typography/"
	t.Run("shared", func(t *testing.T) {
		forbid(t, dependencies(t, "./font", "./fontresource", "./layout", "./style"), root+"glyphing", root+"render")
	})
	t.Run("glyphing", func(t *testing.T) {
		forbid(t, dependencies(t, "./glyphing/..."), root+"render")
	})
	t.Run("render", func(t *testing.T) {
		forbid(t, dependencies(t, "./render/..."), root+"glyphing")
	})
	t.Run("generic", func(t *testing.T) {
		forbid(t, dependencies(t, ".", "./glyphing", "./glyphing/paragraph", "./render", "./font", "./fontresource", "./layout", "./style"),
			"github.com/npillmayer/opentype", root+"glyphing/adapter", root+"render/adapter")
	})
	t.Run("no-discovery", func(t *testing.T) {
		forbid(t, dependencies(t, "./..."), "tytui", "github.com/npillmayer/fontfind", "github.com/npillmayer/w3c-dom")
	})
}
