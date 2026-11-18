#!/usr/bin/env python3
"""Summarize five fresh-process P6 samples; reject missing/failed measurements.

Usage: python3 doc/flow-cost/analyze.py RESULTS_DIRECTORY [NEW_SUMMARY_PATH]
Reads sample-1.log through sample-5.log, creates summary.json (never overwrites). Pilot logs are
deliberately separate: they are not independent repeated measurement evidence.
"""
import json
import math
from pathlib import Path
import statistics
import sys


def expected_names():
    names = set()
    for backend, sizes in [("synthetic", [100, 1000, 10000]), ("gentium", [100, 1000]), ("hebrew", [100, 1000])]:
        for size in sizes:
            names.add(f"BenchmarkP6Prepare/{backend}/N{size}")
            names.add(f"BenchmarkP6Initial/{backend}/N{size}")
            for cut in [10, 50, 90]:
                for operation in ["equal", "continue", "relayout-equal", "relayout-narrow"]:
                    names.add(f"BenchmarkP6Frames/{backend}/N{size}/P{cut}/{operation}")
    for size in [100, 1000, 10000]:
        for operation in ["checkpoint", "phases"]:
            names.add(f"BenchmarkP6FixedSuffix/N{size}/{operation}")
    return names


STREAM_METRICS = {"factories/op", "shapes/op", "acceptance/op"}
SYNTHETIC_METRICS = {"resolutions/op", "metrics/op", "scales/op", "conversions/op"}
CUT_METRICS = {"original-cut-lines/op", "original-cut-percent"}
LEGACY_CUT_METRICS = {"accepted-lines/op", "accepted-percent"}
PHASE_METRICS = {"checkpoint-ns/op", "bind-ns/op", "break-ns/op", "freeze-ns/op", "assembly-ns/op"}
WORK_METRICS = {"bound-knots/op", "prefix-knots/op", "frozen-knots/op", "assembled-lines/op",
                "break-width-queries/op", "prefix-width-queries/op"}


def required_metrics(name):
    """Canonical schema; real faces expose stream counters only."""
    if name not in expected_names():
        raise ValueError(f"unknown benchmark: {name}")
    required = {"ns/op", "B/op", "allocs/op"} | STREAM_METRICS
    if "/synthetic/" in name or name.startswith("BenchmarkP6FixedSuffix/"):
        required |= SYNTHETIC_METRICS
    if name.startswith("BenchmarkP6Frames/"):
        required |= CUT_METRICS
    if name.startswith("BenchmarkP6FixedSuffix/"):
        required |= {"prefix-words/op", "suffix-words/op"}
        if name.endswith("/phases"):
            required |= PHASE_METRICS | WORK_METRICS
    return required


def zero_metrics(name):
    """Hard call gates, independent of speed and allocation measurements."""
    zeros = set() if name.startswith("BenchmarkP6Prepare/") else set(STREAM_METRICS)
    if name.endswith("/equal") or name.endswith("/checkpoint"):
        zeros |= SYNTHETIC_METRICS & required_metrics(name)
    if name.endswith("/phases"):
        zeros.add("prefix-width-queries/op")
    return zeros


def validate_metrics(name, metrics):
    required = required_metrics(name)
    # Preserve dated evidence verbatim, including its historical labels. Accept
    # exactly one complete label pair; never silently combine or rename them.
    if name.startswith("BenchmarkP6Frames/") and LEGACY_CUT_METRICS <= metrics.keys():
        required = (required - CUT_METRICS) | LEGACY_CUT_METRICS
    if metrics.keys() != required:
        raise ValueError(f"metric schema mismatch: {name}: missing={required-metrics.keys()}, extra={metrics.keys()-required}")
    if any(not math.isfinite(v) or v < 0 for v in metrics.values()) or metrics["ns/op"] <= 0:
        raise ValueError(f"invalid metric value: {name}")
    for unit in zero_metrics(name):
        if metrics[unit] != 0:
            raise ValueError(f"zero-call gate failed: {name}: {unit}={metrics[unit]}")


def validate_sample(sample):
    if set(sample) != expected_names():
        raise ValueError(f"matrix mismatch: missing={expected_names()-set(sample)}, extra={set(sample)-expected_names()}")
    cut_schemas = set()
    for name, row in sample.items():
        if row["iterations"] <= 0:
            raise ValueError(f"invalid iterations: {name}")
        validate_metrics(name, row["metrics"])
        if name.startswith("BenchmarkP6Frames/"):
            cut_schemas.add("original-cut-percent" in row["metrics"])
    if len(cut_schemas) != 1:
        raise ValueError("mixed historical and current cut labels in one sample")


def parse_sample(text):
    if "PASS" not in text.splitlines() or any(line.startswith("FAIL") or "--- FAIL:" in line for line in text.splitlines()):
        raise ValueError("incomplete or failed benchmark process")
    result = {}
    for line in text.splitlines():
        if not line.startswith("BenchmarkP6"):
            continue
        parts = line.split()
        if len(parts) < 8 or (len(parts)-2) % 2:
            raise ValueError(f"malformed benchmark row: {line}")
        name, iterations = parts[0], int(parts[1])
        if name in result or iterations <= 0:
            raise ValueError(f"duplicate/invalid benchmark: {name}")
        metrics = {}
        for i in range(2, len(parts), 2):
            value, unit = float(parts[i]), parts[i+1]
            if not math.isfinite(value) or value < 0 or unit in metrics:
                raise ValueError(f"invalid metric: {line}")
            metrics[unit] = value
        validate_metrics(name, metrics)
        result[name] = {"iterations": iterations, "metrics": metrics}
    validate_sample(result)
    return result


def distribution(values):
    mean = statistics.mean(values)
    return {"median": statistics.median(values), "min": min(values), "max": max(values),
            "cv_percent": 100*statistics.pstdev(values)/mean if mean else 0, "samples": values}


def weighted(equal, different, share):
    if len(equal) != len(different) or not 0 <= share <= 1:
        raise ValueError("invalid workload mixture")
    return [share*a + (1-share)*b for a, b in zip(equal, different)]


def summarize(samples):
    if len(samples) != 5:
        raise ValueError("five independent process samples required")
    for sample in samples:
        validate_sample(sample)
    summary = {}
    for name in sorted(expected_names()):
        units = samples[0][name]["metrics"].keys()
        if any(s[name]["metrics"].keys() != units for s in samples):
            raise ValueError(f"inconsistent metrics: {name}")
        summary[name] = {unit: distribution([s[name]["metrics"][unit] for s in samples]) for unit in units}
        summary[name]["iterations"] = [s[name]["iterations"] for s in samples]
    mixtures = {}
    for name in sorted(summary):
        if not name.startswith("BenchmarkP6Frames/") or not name.endswith("/equal"):
            continue
        prefix = name.removesuffix("/equal")
        for share in [.9, .5]:
            group = {}
            for unit in ["ns/op", "B/op", "allocs/op"]:
                values = lambda operation: summary[prefix+"/"+operation][unit]["samples"]
                chosen = weighted(values("equal"), values("continue"), share)
                baseline = weighted(values("relayout-equal"), values("relayout-narrow"), share)
                group[unit] = {"retained": distribution(chosen), "relayout": distribution(baseline)}
                if unit == "ns/op":
                    group["time_ratio"] = distribution([b/a for a, b in zip(chosen, baseline)])
            mixtures[f"{prefix}/equal-share-{share:.1f}"] = group
    return {"sample_count": 5, "case_count": len(summary), "benchmarks": summary, "mixtures": mixtures}


if __name__ == "__main__":
    if len(sys.argv) not in (2, 3):
        sys.exit(__doc__)
    directory = Path(sys.argv[1])
    samples = [parse_sample((directory/f"sample-{i}.log").read_text()) for i in range(1,6)]
    result = summarize(samples)
    destination = Path(sys.argv[2]) if len(sys.argv) == 3 else directory/"summary.json"
    with destination.open("x") as output:
        output.write(json.dumps(result, indent=2)+"\n")
    print(f"Summarized {result['case_count']} cases from five complete process samples")
