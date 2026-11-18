# integration

Tests crossing typography's independent glyphing and rendering branches. This
is a test-only package, not a production API or application adapter. Generic
packages and their concrete adapters must not import the opposite branch just
to provide test fixtures; compose the branches here instead.

The suite covers:

- Loading/itemization through paragraph layout into retained glyph-outline proofs.
- Exact line/baseline coordinates, source/paint correspondence, and immutable
  layout publication with synthetic services and offline real fonts.
- Retained resource lifetime, concurrent outlines and independent render services.
- Fragment proofs across width changes, RTL/empty fragments, failure/cancellation,
  and rendering after working paragraph preparation has been collected.

Proof construction must not reload files, reshape, rebreak or query line metrics.
Frozen output shares stable font/source resources while owning detached geometry.
Paragraph preparation and checkpoints are released at completion or abandonment;
retaining a fragment alone must not retain them.

From the module root:

```sh
go test ./integration -count=1
go test -race ./integration -count=1
```

Real-font fixtures come from the sibling opentype checkout through the ordinary
adapters; do not download fonts or relax production parsing. Local checks for
individual packages remain in those packages. Root dependency audits enforce the
branch boundaries. Document/CSS composition, frame scheduling, CLI output and
replay-v5 controls are tested in sibling tytui, especially `documentlayout` and
`tycli`; they do not belong in the shared typography model.

See the [module overview](../README.md),
[paragraph flow and lifecycle](../glyphing/paragraph/README.md#checkpoint-based-frame-flow),
[retained rendering](../render/README.md) and
[production cost report](../doc/FLOW_COSTS.md).
