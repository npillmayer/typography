# layout

Completed immutable paragraph data, with no layout or rendering algorithms.

NewVList validates finite geometry and frozen line/atomic associations and copies
all input slices. Accessors return detached geometry and the shared immutable
frozen content. Empty placements can retain diagnostic frozen content for
suppressed empty source.

Coordinates are fractional big points, Y downward. Baselines and atomic origins
can be signed. Logical boxes do not bound glyph ink. Material distinguishes line
boxes from fixed kerns. Atomic placements preserve physical traversal, not X sort.

Glyphing's paragraph assembler owns mutable construction state and publishes
only completed results. Rendering never rebuilds metrics or changes placements.
This package is not a page-layout engine or wire format.
