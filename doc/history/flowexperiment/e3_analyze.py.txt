#!/usr/bin/env python3
"""Summarize five fresh-process E3 samples; reject missing/failed measurements.

Usage: python3 internal/flowexperiment/e3_analyze.py RESULTS_DIRECTORY
Reads sample-1.log through sample-5.log, writes summary.json. Pilot logs are
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
            names.add(f"BenchmarkE3Prepare/{backend}/N{size}")
            for cut in [10, 50, 90]:
                for operation in ["equal", "reference", "resumed", "relayout-equal", "relayout-narrow"]:
                    names.add(f"BenchmarkE3Frames/{backend}/N{size}/P{cut}/{operation}")
    for size in [100, 1000, 10000]:
        for operation in ["checkpoint", "reference", "resumed"]:
            names.add(f"BenchmarkE3FixedSuffix/N{size}/{operation}")
    return names


def parse_sample(text):
    if "PASS" not in text.splitlines() or any(line.startswith("FAIL") or "--- FAIL:" in line for line in text.splitlines()):
        raise ValueError("incomplete or failed benchmark process")
    result = {}
    for line in text.splitlines():
        if not line.startswith("BenchmarkE3"):
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
        if not {"ns/op", "B/op", "allocs/op"} <= metrics.keys():
            raise ValueError(f"missing timing/allocation metrics: {name}")
        result[name] = {"iterations": iterations, "metrics": metrics}
    if set(result) != expected_names():
        raise ValueError(f"matrix mismatch: missing={expected_names()-set(result)}, extra={set(result)-expected_names()}")
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
    summary = {}
    for name in sorted(expected_names()):
        units = samples[0][name]["metrics"].keys()
        if any(s[name]["metrics"].keys() != units for s in samples):
            raise ValueError(f"inconsistent metrics: {name}")
        summary[name] = {unit: distribution([s[name]["metrics"][unit] for s in samples]) for unit in units}
        summary[name]["iterations"] = [s[name]["iterations"] for s in samples]
    mixtures = {}
    for name in sorted(summary):
        if not name.startswith("BenchmarkE3Frames/") or not name.endswith("/equal"):
            continue
        prefix = name.removesuffix("/equal")
        for share in [.9, .5]:
            group = {}
            for unit in ["ns/op", "B/op", "allocs/op"]:
                values = lambda operation: summary[prefix+"/"+operation][unit]["samples"]
                chosen = weighted(values("equal"), values("resumed"), share)
                baseline = weighted(values("relayout-equal"), values("relayout-narrow"), share)
                group[unit] = {"retained": distribution(chosen), "relayout": distribution(baseline)}
                if unit == "ns/op":
                    group["time_ratio"] = distribution([b/a for a, b in zip(chosen, baseline)])
            mixtures[f"{prefix}/equal-share-{share:.1f}"] = group
    return {"sample_count": 5, "case_count": len(summary), "benchmarks": summary, "mixtures": mixtures}


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    directory = Path(sys.argv[1])
    samples = [parse_sample((directory/f"sample-{i}.log").read_text()) for i in range(1,6)]
    result = summarize(samples)
    (directory/"summary.json").write_text(json.dumps(result, indent=2)+"\n")
    print(f"Summarized {result['case_count']} cases from five complete process samples")
