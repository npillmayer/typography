# E3 measurement evidence, 2026-11-18

The [decision report](../../../../E3_RESULTS.md) interprets these measurements.
They describe the experimental overlay, not production timing guarantees.

| Artifact | Content |
| --- | --- |
| [environment.json](environment.json) | Go/OS/configuration, exact command, repository revisions/status, source and overlay hashes, font hashes |
| [sample-1.log](sample-1.log), [sample-2.log](sample-2.log), [sample-3.log](sample-3.log), [sample-4.log](sample-4.log), [sample-5.log](sample-5.log) | Five fresh sequential Go test processes, 121 cases each, all successful |
| [summary.json](summary.json) | All five values, median, min/max and population CV for each metric; paired workload mixtures |
| [pilot-large-real.log](pilot-large-real.log) | Earlier successful single-iteration pilot, including real-font 10,000-word cases; excluded from repeated-sample statistics |

The CPU header in each log records Apple M1. The run used Go 1.25.6 on macOS
26.3, darwin/arm64, `GOMAXPROCS=1` (`-cpu=1`), without race instrumentation
or concurrent verification jobs. CPU frequency/affinity and other host
applications were not controlled. There are no excluded outliers or failed
benchmark processes in the five-sample set.

The source/overlay hashes were captured before sampling. Final documentation,
including the package comment in `internal/flowexperiment/doc.go`, was refreshed
afterward; executable Go sources, overlay assets and measurement scripts were
unchanged. The sibling checkouts supply local dependencies and fonts; their
revisions and dirty state are recorded rather than assumed clean.

## Repeat the bounded matrix

Run from the typography root, with sibling khipu/opentype and their existing
offline fixtures available. Use a fresh output directory; retain previous
evidence. The scripts need Python 3 and the Go toolchain, with no extra Python
dependencies. The overlay generator fails on unexpected source anchors.

```sh
python3 internal/flowexperiment/e3_overlay.py /tmp/flow-e3
go test -overlay /tmp/flow-e3/overlay.json ./internal/flowexperiment -count=1
python3 -B internal/flowexperiment/e3_analyze_test.py
```

Record provenance, then run five sequential processes. The commands below use
a new directory; choose another if it already contains results. The recorder
refuses to overwrite an existing `environment.json`.

```sh
flow_e3_results=/tmp/flow-e3-new-results
python3 internal/flowexperiment/e3_record.py /tmp/flow-e3/overlay.json "$flow_e3_results"
for flow_e3_sample in 1 2 3 4 5; do
    go test -overlay /tmp/flow-e3/overlay.json ./internal/flowexperiment \
        -run '^$' -bench '^BenchmarkE3' -benchmem -benchtime=50ms \
        -count=1 -cpu=1 -timeout=10m \
        > "$flow_e3_results/sample-$flow_e3_sample.log" 2>&1 || break
done
python3 -B internal/flowexperiment/e3_analyze.py "$flow_e3_results"
```

Do not substitute `-count=5` in one process or run samples alongside tests.
The analyzer requires all five complete logs and the exact 121-case matrix. It
rejects failed, incomplete, missing, duplicate or malformed benchmark results.
Each process uses Go's benchmark calibration; a 50 ms target may produce just
one timed iteration for expensive preparation. Timing and allocation distributions
are summarized across processes, not by treating calibration iterations as
independent samples.

## Separate large-font pilot

The saved pilot used the same benchmark command with `-benchtime=1x` and
`-timeout=3m`, before the repeated real-font matrix was bounded and before the
additional synthetic metric counters were reported. Its default selection then
included 10,000-word real-font cases. The current equivalent selection requires
`-flow-e3-large-real`; this is a new pilot, not an exact replay of the earlier
instrumentation:

```sh
go test -overlay /tmp/flow-e3/overlay.json ./internal/flowexperiment \
    -run '^$' -bench '^BenchmarkE3' -benchmem -benchtime=1x \
    -count=1 -cpu=1 -timeout=10m -flow-e3-large-real
```

Gentium's 10,000-word preparation alone took 20.57 seconds and allocated 17.02 GB
cumulatively in the original pilot. This is allocation traffic, not peak memory.
The repeated set therefore limits both real backends to 100/1,000 words while
retaining all three scales synthetically. The pilot is separate evidence and
must not be passed to the standard matrix analyzer or folded into its averages.
