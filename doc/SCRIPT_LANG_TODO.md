# TODO: Script/language-aware font selection

Status: open, recorded 2026-11-17. No selection-policy change is implemented by
this task.

- [ ] Extend `opentype/otquery` with a precise script/language capability query,
  then use it through typography's OpenType adapter to help select an appropriate
  preloaded font for the effective style and current text script/language.

## Current behavior

The pipeline determines script and language but does not ask each candidate font
whether it explicitly supports them before selecting it:

- The itemizer determines script from the text and applies its Latin/Hebrew
  restriction.
- The resolver obtains language from the effective style.
- Font selection checks nominal glyph coverage for each grapheme.
- Script and language are passed to the shaper after selection.

OpenType's [plan compiler](../../opentype/otshape/plan_compile.go) selects the
script/language systems for shaping, falling back to the `DFLT` script and the
selected script's default language system where applicable.

The existing [`otquery.FontSupportsScript`](../../opentype/otquery/metrics.go)
inspects **GSUB only** and returns script/language tags or default markers. Those
markers do not establish that an actual default system exists. The query does
not inspect GPOS, verify character coverage, or establish shaping correctness.
The typography pipeline currently does not call it.

The current selection criterion is therefore **nominal character coverage**.
Absence of an explicit language system does not mean that the font cannot typeset
that language: a default system may suffice. Conversely, explicit script/language
entries do not guarantee glyph coverage or correct shaping. Do not use the
existing query as a boolean acceptance gate.

## Work to do

- [ ] Define and implement a richer `opentype/otquery` result that distinguishes
  explicit script/language systems, actual default-system fallback, absent
  systems/tables, and malformed data. Inspect GSUB and GPOS separately and report
  the selected systems consistently with the plan compiler's fallback rules.
  Define how language tags map to OpenType language-system tags, including
  unspecified or unmapped languages. Keep table evidence distinct from a promise
  of shaping correctness.
- [ ] Expose this evidence through an injected, backend-neutral capability
  service consumed by root typography. Keep concrete OpenType queries and type
  assertions inside `adapter/opentype`, under the existing shared face lock.
  Reuse existing font/script/language contracts where suitable; do not expose a
  parsed-font accessor or couple the generic resolver to OpenType.
- [ ] Define the selection policy before changing behavior: when to prefer an
  explicit system, when default systems suffice, and how absent/unknown evidence
  affects candidate ranking or rejection. Consider the style-mapped face and
  explicitly configured fallbacks while retaining nominal grapheme coverage as
  a separate requirement. Resolve Common/Inherited script context consistently
  before making script-dependent choices. Expanding the Latin/Hebrew restriction
  is a separate decision.
- [ ] Integrate the policy during itemization/selection, without discovery or
  loading. Preserve grapheme boundaries, source coordinates, feature domains,
  paint separation, deterministic candidate ordering, and terminal query errors.
  Resolver construction must remain free of font queries.

This capability query is **separate from `CheckFace`**. `CheckFace` remains a cheap
representation-compatibility check with no font-table queries or stream mutation.
For OpenType it remains adapter-only and must not be forwarded to `otshape`.

## Acceptance and verification

- [ ] Test explicit and default script/language selection, missing defaults,
  GSUB/GPOS disagreement, malformed tables, unknown language mappings, and fonts
  whose nominal coverage and layout-table evidence differ. Place query tests in
  OpenType and selection-policy tests with fake services in typography.
- [ ] Add tytui integration cases for style mapping and explicit fallback,
  including offline replay. Script-aware selection may intentionally change
  glyphs or placements: review affected current-v4 evidence explicitly rather
  than automatically regenerating fixtures. Preserve immutable resources,
  provenance/identity encoding and historical-version rejection cases; serialize
  no runtime services. Assess any necessary replay-policy/schema change before
  implementation under the current experimental replay contract.
- [ ] Run the affected OpenType checks and typography/tytui ordinary and race
  suites, build, vet, dependency audits and adapter regressions. Include relevant
  khipu/css-box-layout checks if shared contracts or pipeline behavior change.
