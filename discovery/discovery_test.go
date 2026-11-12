package discovery_test

import (
	"context"
	"errors"
	"github.com/npillmayer/khipu/helpers"
	"github.com/npillmayer/typography"
	"github.com/npillmayer/typography/discovery"
	"github.com/npillmayer/typography/fontresource"
	"strings"
	"testing"
)

type search struct {
	calls    int
	requests []fontresource.Request
	readErr  bool
}

func (*search) Names() []string { return []string{"fake"} }
func (s *search) Search(ctx context.Context, mapped, original fontresource.Request, allow bool) (discovery.SearchResult, error) {
	s.calls++
	s.requests = append(s.requests, mapped)
	if s.readErr {
		return discovery.SearchResult{}, &discovery.ResolutionError{Stage: "read", Err: errors.New("read failed")}
	}
	if mapped.Pattern == "absent" {
		return discovery.SearchResult{}, errors.New("not found")
	}
	r, _ := fontresource.Read(strings.NewReader("not a real font"), fontresource.Origin{Provider: "fake", Location: "memory", Request: original})
	return discovery.SearchResult{Resource: r, Provider: "fake", Diagnostics: []discovery.FontDiagnostic{{Stage: "lookup", Request: original, Provider: "fake", Message: "exact match"}}}, nil
}
func TestPolicyUsesInjectedValidationAndIsolatedCache(t *testing.T) {
	search := &search{}
	loads := 0
	loader := typography.LoadFunc(func(ctx context.Context, _ string, r *fontresource.Resource) (typography.FaceEntry, error) {
		loads++
		return typography.FaceEntry{Face: helpers.NewSyntheticFont(), Resource: r, Info: typography.Info{ResourceID: r.ID(), SHA256: r.SHA256(), Origin: r.Origin(), Path: r.Origin().Location, Family: "fake"}}, nil
	})
	generics := map[string]string{"serif": "mapped"}
	d, err := discovery.New(search, loader, generics)
	if err != nil {
		t.Fatal(err)
	}
	generics["serif"] = "changed"
	req := discovery.FontRequest{Families: []string{"absent", "serif"}}
	r, ds, err := d.Resolve(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if loads != 1 || search.calls != 2 || search.requests[1].Pattern != "mapped" || r.Origin().Request.Pattern != "serif" {
		t.Fatal("policy/loader mismatch")
	}
	ds[0].Message = "mutated"
	cached, ds, err := d.Resolve(context.Background(), req)
	if err != nil || cached != r || search.calls != 2 || loads != 1 || ds[0].Message == "mutated" {
		t.Fatal("cache lost ownership", err)
	}
	other, _ := discovery.New(search, loader, nil)
	if _, _, err = other.Resolve(context.Background(), req); err == nil {
		t.Fatal("cache crossed sessions")
	}
}
func TestReadAndParseFailuresAreTerminal(t *testing.T) {
	for _, readFail := range []bool{false, true} {
		s := &search{readErr: readFail}
		loads := 0
		loader := typography.LoadFunc(func(context.Context, string, *fontresource.Resource) (typography.FaceEntry, error) {
			loads++
			return typography.FaceEntry{}, errors.New("parse failed")
		})
		d, _ := discovery.New(s, loader, nil)
		for i := 0; i < 2; i++ {
			_, _, err := d.Resolve(context.Background(), discovery.FontRequest{Families: []string{"first", "second"}})
			var stage *discovery.ResolutionError
			if !errors.As(err, &stage) {
				t.Fatal(err)
			}
			want := "parse"
			if readFail {
				want = "read"
			}
			if stage.Stage != want {
				t.Fatal(stage)
			}
		}
		if s.calls != 2 || (!readFail && loads != 2) || (readFail && loads != 0) {
			t.Fatal("failure was cached or fell through")
		}
	}
}
