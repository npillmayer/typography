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

func TestExtractedPackageDependencyBoundaries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "go", "list", "-deps", "-json", ".", "./style", "./fontresource", "./render", "./paragraph").CombinedOutput()
	if err != nil {
		t.Fatalf("dependency inspection: %v: %s", err, output)
	}
	dec := json.NewDecoder(strings.NewReader(string(output)))
	for {
		var pkg struct {
			ImportPath string
			Imports    []string
		}
		if err = dec.Decode(&pkg); err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("dependency inspection: %v: %s", err, output)
		}
		for _, blocked := range []string{"tytui", "github.com/npillmayer/w3c-dom", "github.com/npillmayer/fontfind", "github.com/npillmayer/opentype", "github.com/npillmayer/typography/adapter", "github.com/npillmayer/typography/discovery"} {
			if pkg.ImportPath == blocked || strings.HasPrefix(pkg.ImportPath, blocked+"/") {
				t.Fatalf("generic packages acquired dependency %s", pkg.ImportPath)
			}
		}
	}
}
