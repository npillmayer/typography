#!/usr/bin/env python3
"""Add E3 clocks and benchmarks to the E2 overlay, without checkout edits.

Usage: python3 internal/flowexperiment/e3_overlay.py /tmp/flow-e3
"""
import contextlib
import io
import json
from pathlib import Path
import runpy
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / "internal/flowexperiment/testdata/e3"


def generate(destination):
    e2 = runpy.run_path(str(ROOT / "internal/flowexperiment/e2_overlay.py"))
    replace = e2["replace"]
    with contextlib.redirect_stdout(io.StringIO()):
        e2["generate"](destination)
    overlay = destination / "overlay.json"
    mapping = json.loads(overlay.read_text())["Replace"]

    def read(target):
        return Path(mapping.get(str(target), target)).read_text()

    def emit(target, source):
        path = destination / f"e3-{len(mapping):02}-{target.name}"
        path.write_text(source)
        subprocess.run(["gofmt", "-w", str(path)], check=True)
        mapping[str(target)] = str(path)

    # Clock only phase boundaries. No timer/counter is added inside a knot loop.
    # Without a timing context these helpers do not read the clock.
    for name in ["prepared.go", "zz_flow_e2.go"]:
        target = ROOT / "glyphing/paragraph" / name
        source = read(target)
        anchor = "bound, err := p.owner.BindParagraph(ctx, p.content, advances)"
        source = replace(source, anchor, "flowStamp := flowClock(ctx)\n" + anchor + '\nflowRecord(ctx, flowStamp, "bind")')
        if name == "zz_flow_e2.go":
            anchor = "breaks, visits, err := firstfit.FlowBreak(ctx, bound, geometry, prefix, resume)"
            source = replace(source, anchor, "flowStamp = flowClock(ctx)\n" + anchor + '\nflowRecord(ctx, flowStamp, "break")')
        emit(target, source)
    target = ROOT / "glyphing/paragraph/layout.go"
    source = read(target)
    source = replace(source, "switch params.Breaker {", "flowStamp := flowClock(ctx)\nswitch params.Breaker {")
    anchor = "\tif err != nil {\n\t\treturn fail(StageBreak, err)\n\t}\n\tif err := ctx.Err(); err != nil {"
    source = replace(source, anchor, '\tflowRecord(ctx, flowStamp, "break")\n' + anchor)
    anchor = "frozen, err := owner.Freeze(ctx, encoded, breaks, geometry)"
    source = replace(source, anchor, "flowStamp := flowClock(ctx)\n" + anchor + '\nflowRecord(ctx, flowStamp, "freeze")')
    anchor = "vlist, err := assembleLineHeight(ctx, frozen, params.LineLayout, text, config, atomics)"
    source = replace(source, anchor, "flowStamp = flowClock(ctx)\n" + anchor)
    anchor = "published, err := vlist.publish()"
    source = replace(source, anchor, anchor + '\nflowRecord(ctx, flowStamp, "assembly")')
    emit(target, source)
    emit(ROOT / "glyphing/paragraph/zz_flow_e3.go", (ASSETS / "timing.go.txt").read_text())

    # Reuse E2's correctness oracle during untimed benchmark admission.
    target = ROOT / "internal/flowexperiment/e2_continuation_test.go"
    source = read(target)
    for name in ["compareFlow", "compareCompleted", "compareKnot", "noBaseShaping"]:
        source = replace(source, f"func {name}(t *testing.T,", f"func {name}(t testing.TB,")
    emit(target, source)
    for asset in sorted(ASSETS.glob("*_test.go.txt")):
        emit(ROOT / "internal/flowexperiment" / ("e3_" + asset.name.removesuffix(".txt")), asset.read_text())
    overlay.write_text(json.dumps({"Replace": mapping}, indent=2) + "\n")
    print(overlay)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    generate(Path(sys.argv[1]).resolve())
