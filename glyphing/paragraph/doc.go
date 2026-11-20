// Package paragraph prepares styled text, measures intrinsic widths and performs
// line breaking, freezing and CSS line-height assembly with injected services.
// Fragment, SplitFragment and FitFragment reuse completed geometry without
// shaping, metric queries or retained working preparation. Styles and bidi
// context stay paragraph-relative; paint and surrounding margins are external.
// Prepared.Trial retains independent immutable decisions and matching assembly
// inputs for later Trial.Assemble. Trial.Checkpoint locks a compatible prefix
// for a first-fit suffix trial; it does not schedule frames or implement a page
// builder. Keep preparation only while further searches/measurement are needed.
// Prepared.StartFlow and Continuation.Fit support NoHyphenation frame flow:
// equal widths reuse geometry, changed widths lock accepted boundaries and fully
// assemble a first-fit suffix using the retained preparation. Drop working tokens
// after each paragraph finishes; completed fragments need no preparation handle.
package paragraph
