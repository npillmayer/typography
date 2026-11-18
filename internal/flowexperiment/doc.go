// Package flowexperiment hosts the bounded paragraph continuation experiment
// described in README.md and doc/FLOW.md.
//
// Stage E1 partitions compatible equal-width frozen lines into two frames and an
// unplaced tail, preserving immutable ownership and full CSS line heights. It
// performs geometry work only. Helpers are private to the experimental harness;
// this package provides no production API or application integration. Stage E2
// uses temporary Go overlays to test fixed-prefix first-fit continuation against
// a constrained reference. Stage E3 measures costs through an additional overlay
// and records the decision in E3_RESULTS.md. Production integration, the upstream
// bidi fix and elasticity remain deferred.
package flowexperiment
