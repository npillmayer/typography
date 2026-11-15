# discovery

Neutral ordered font selection with explicit search/loading services, generic-family
policy, session caching and diagnostic evidence. No backend or provider is installed.

```go
session, err := discovery.New(searcher, loader, map[string]string{
    "serif": "Example Serif",
})
if err != nil {
    return err
}
resource, diagnostics, err := session.Resolve(ctx, discovery.FontRequest{
    Families: []string{"serif"}, Weight: 400, Slant: "normal",
})
// Inspect diagnostics even when err is nonnil; consume resource only on success.
```

`Descriptors` converts supported CSS weights (100–900 in steps of 100, zero means
400) and slants into legacy request codes. `Resolve` preserves family/provider
order, requires explicit generic mappings and reports opt-in substitution. Read
and parse failures terminate selection. Only successful results are cached,
including their diagnostics; returned diagnostic slices are detached.

`ResolveSettings` resolves default, rule and fallback requests into `Resolution`,
which carries settings and application provenance without backend services.
`Clone` methods detach configuration/evidence while sharing immutable resources.

Calls on a session are serialized through a cancellable gate. Keep search/loading
services stable; providers must be offline, concurrency-safe and honor cancellation.
Create a new session when policy changes. Glyph coverage fallback belongs to the
root itemizer, not discovery.

Run `go test ./discovery`; neutral tests use fake services. Provider integration
is tested in `adapter/fontfind`. See the [integration guide](../README.md).
