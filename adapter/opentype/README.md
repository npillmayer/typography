# adapter/opentype

This package exposes implementation of `khipu` and `typography` service contracts.
It does this with OpenType fonts, based on package `github.com/npillmayer/opentype`.
It owns normal parsing, retained bytes, nominal coverage, metrics/scales,
streaming shaping, identity and outline conversion. No public parsed-font accessor
or implicit system/network discovery is provided.

```go
services := opentypeadapter.Services{}
entry, err := services.Load(ctx, path, nil)
if err != nil { return err }
resolver, err := typography.NewResolver(ctx, typography.SelectedFonts{
    Faces: []typography.FaceEntry{entry}, Default: entry.Info.ResourceID, Size: size,
}, services)
// Handle err before using resolver; retain services/faces for result lifetimes.
```

Here `opentypeadapter` aliases this package. `Services` also implements
`typography.IdentityProvider` and `render.Backend`. `LoadFile` snapshots a regular
file; `LoadResource` uses an immutable snapshot; `Loader.LoadFont` adapts an
`io.Reader` to khipu's loader contract. All use normal parser policy. Identity
and outlines use retained bytes rather than reopening paths.

Metrics are in font units. `Face.Scale` accepts a positive finite DU em size;
`Scale.ToDU` returns fractional DU. Outlines use font units with Y downward and
are converted to SVG paths/control points by `Services.Outline`. Lazy coverage,
shaping and outline work share the face lock. Identity metadata is detached.

## Face acceptance

`Stream.CheckFace` and `Services.CheckFace` accept initialized `*Face` instances.
They share private validation with `Shape` and coverage lookup, reject foreign
representations with `contracts.ErrUnsupportedFontFace`, and reject nil or
uninitialized faces with `contracts.ErrIllegalArguments`. Checks inspect only
immutable preparation state: no parsing, queries, stream mutation or lock-held
backend work. They are safe concurrently and independent of stream readiness.
No public parsed-font accessor or renderer compatibility contract is introduced.

## Streaming lifecycle

`NewStream` requires positive channel capacity and starts no goroutines. Arrange
concurrent draining of `Glyphs()` before calling synchronous `Shape`; otherwise
bounded output can block production. Use a fresh exclusive stream where required
by khipu's streaming factory contract.

`SetInitialCluster` sets a scalar base. Successful `Shape` calls advance it by
input rune count and leave the channel open for more runs. Features and clusters
use scalar positions, not UTF-8 bytes. Busy/closed operations fail. Once a run
starts, validation, shaping or cancellation errors are terminal, retained by
`Err`, and close output. `Finish` closes an idle stream, is idempotent after
closure, and returns the retained error. It also releases an unused idle stream
without emitting glyphs or needing a drainer. Synchronous backend work has cancellation
limits even though reads and output check the context.

`VisualOrder` operates on caller-owned directional fragments; frozen knot order
must remain physical. Keep font/scale services valid for retained layout/proof
results, and never substitute relaxed test parsing in production.

Run `go test ./adapter/opentype` and `go test -race ./adapter/opentype`. Tests use
sibling opentype font fixtures and cover parsing, retained resources, shaping,
feature coordinates, prepared reuse and rendering. See the
[integration guide](../../README.md) for cross-module checks.
