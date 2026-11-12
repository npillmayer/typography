package fontresource

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type infiniteReader struct{ read int }

func (r *infiniteReader) Read(p []byte) (int, error) { clear(p); r.read += len(p); return len(p), nil }
func TestBoundedReads(t *testing.T) {
	if _, err := Read(nil, Origin{}); err == nil {
		t.Fatal("nil reader")
	}
	if _, err := Read(failingReader{}, Origin{}); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatal(err)
	}
	large := &infiniteReader{}
	if _, err := Read(large, Origin{}); err == nil || large.read != MaxBytes+1 {
		t.Fatalf("unbounded read %d: %v", large.read, err)
	}
}
func TestOwnershipAndIdentity(t *testing.T) {
	data := []byte("font snapshot")
	origin := Origin{Provider: "offline", Name: "Example", Location: "fonts/example.ttf", Request: Request{Pattern: "Example", Style: Italic, Weight: Bold}}
	r, err := Read(bytes.NewReader(data), origin)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	origin.Name = "mutated"
	got, err := io.ReadAll(r.Open())
	if err != nil || string(got) != "font snapshot" {
		t.Fatal("snapshot aliased input", err)
	}
	clear(got)
	again, _ := io.ReadAll(r.Open())
	if string(again) != "font snapshot" {
		t.Fatal("reader exposed mutable bytes")
	}
	// Fixed version-1 encoding, computed independently from the former implementation.
	if r.SHA256() != "7091bf0607d62ef1668d988eb9e5cf0840a5ba64086414e90bc1bbb8f38f5333" || r.ID() != "sha256:98228883a67d77d5ec5b73c376f473dcb7fbda0334f72610ab1d3440384e92b4" {
		t.Fatalf("identity changed: %s %s", r.SHA256(), r.ID())
	}
	changed := r.Origin()
	changed.Location = "elsewhere"
	other, _ := Read(bytes.NewReader(again), changed)
	if other.SHA256() != r.SHA256() || other.ID() == r.ID() {
		t.Fatal("content/provenance identities conflated")
	}
}
