# adapter/fontfind

This package provides an adapter for package `github.com/pillmayer/fontfind`.
It makes it accessible for neutral  discovery requests.
It owns enum conversion, a private resolver pipeline and immutable snapshots;
`discovery` owns session policy/cache and requires a separately injected loader.

```go
searcher, err := fontfindadapter.New([]fontfindadapter.OfflineProvider{
    {Name: "bundled", Find: findBundled},
})
if err != nil {
    return err
}
session, err := discovery.New(searcher, loader, genericFamilies)
// Handle err before using session.
```

Here `fontfindadapter` aliases this package; `findBundled` implements the callback
signature of `OfflineProvider.Find`. Providers are tried in supplied order and
must have unique nonempty names, nonnil callbacks, offline trusted resources,
concurrency safety and cancellation support.

`Match.Exact` is an explicit attestation. A nil error does not prove an exact
match; an unattested or error-bearing font requires substitution permission.
`Search` uses its mapped request for lookup and original request for provenance,
returns ordered evidence, and disables implicit packaged fallback. Read failures
are terminal. Each call owns a fresh registry and result evidence.

`Descriptor`/`Request` convert legacy enum codes, not numeric CSS weights, and do
not validate them. `ReadResource` snapshots bytes, but fontfind's `ReadFontData`
allocation occurs before the 64 MiB bound: only trusted providers belong here.

Run `go test ./adapter/fontfind` for controlled-provider, matching, cancellation
and cache-integration regressions. See the [integration guide](../../README.md).
