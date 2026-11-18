#!/usr/bin/env python3
"""Generate reproducible E2 Go overlays; never write to an owning package.

Usage: python3 internal/flowexperiment/e2_overlay.py /tmp/flow-e2
Then: go test -overlay /tmp/flow-e2/overlay.json ./...
Use the same absolute overlay path from sibling khipu and tytui.
"""
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / "internal/flowexperiment/testdata/e2"
KHIPU = ROOT.parent / "khipu"


def replace(text, old, new):
    if text.count(old) != 1:
        raise RuntimeError(f"upstream seam changed; review patch anchor: {old[:100]!r}")
    return text.replace(old, new)


def generate(destination):
    if destination.is_relative_to(ROOT) or destination.is_relative_to(KHIPU):
        raise ValueError("keep generated seams outside the typography/khipu checkouts")
    destination.mkdir(parents=True, exist_ok=True)
    mapping = {}

    def emit(target, text):
        path = destination / f"{len(mapping):02}-{target.name}"
        path.write_text(text)
        subprocess.run(["gofmt", "-w", str(path)], check=True)
        mapping[str(target)] = str(path)

    # Modify the owning first-fit loop in place in the overlay. Its ordinary
    # entry remains identical: nil control means the existing whole-paragraph run.
    path = KHIPU / "linebreak/firstfit/break.go"
    text = path.read_text()
    signature = "func BreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters) ([]kinx, error) {"
    production_seed = "func breakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters, seed *breakSeed) ([]kinx, error) {"
    if production_seed in text:
        # P2 shares the scan with a production seed. Keep both public production
        # entries intact and add E2 control only in this temporary overlay.
        text = replace(text, production_seed, production_seed + """
 return flowBreakWithSeed(khp, layout, params, seed, nil)
}
func flowBreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters, flow *flowControl) ([]kinx,error) {
 return flowBreakWithSeed(khp, layout, params, nil, flow)
}
func flowBreakWithSeed(khp *khipu.Khipu, layout khipu.Params, params *Parameters, seed *breakSeed, flow *flowControl) ([]kinx,error) {
""")
    else:
        text = replace(text, signature, signature + "\n return flowBreakParagraph(khp, layout, params, nil)\n}\n\nfunc flowBreakParagraph(khp *khipu.Khipu, layout khipu.Params, params *Parameters, flow *flowControl) ([]kinx,error) {")
    text = replace(text, "trace := tracer()", ("" if production_seed in text else "start := 0\n") + """
 if flow != nil && flow.resume && len(flow.prefix)>0 {
  breakpoints = append(breakpoints,flow.prefix...)
  start = flow.prefix[len(flow.prefix)-1]+1
 }
 trace := tracer()""")
    # With P2 the start variable and loop already exist. The instrumentation
    # still counts actual visits, independently of production's cancellation checks.
    if production_seed not in text:
        text = replace(text, "for i := range len(khp.Kind) {", "for i := start; i < len(khp.Kind); i++ {")
    text = replace(text, "k := khp.KnotByIndex(i)", """
  if flow != nil {
   if err:=flow.ctx.Err();err!=nil{return nil,err}
   flow.visits.Search++
  }
  k := khp.KnotByIndex(i)
  if flow != nil && len(breakpoints)<len(flow.prefix) {
   // Constrained reference: visit every prefix knot and obey exactly the
   // already certified boundaries. No new optional/forced decision is made.
   flow.visits.PrefixSearch++
   seg.append(k)
   if i==flow.prefix[len(breakpoints)] {
    breakpoints=append(breakpoints,i)
    seg.clear()
   }
   continue
  }""")
    text = replace(text, "clear(khp.SelectedDiscretionaries)", "if flow!=nil {if err:=flow.ctx.Err();err!=nil{return nil,err}}\n clear(khp.SelectedDiscretionaries)")
    emit(path, text)

    # Observe real loops; counters are scoped to the continuation context.
    path = KHIPU / "paragraph_input.go"
    emit(path, replace(path.read_text(), "for i, core := range prepared.knots {", "for i, core := range prepared.knots {\n flowBound(ctx,i)"))
    path = KHIPU / "logical_lines.go"
    emit(path, replace(path.read_text(), "for i := previous + 1; i <= at; i++ {", "for i := previous + 1; i <= at; i++ {\n flowMaterialized(ctx,i)"))

    # Retain the original elastic breakpoint indexes privately, and share the
    # existing freeze/CSS publication tail with the experimental continuation.
    path = ROOT / "glyphing/paragraph/layout.go"
    text = replace(path.read_text(), "type Result struct {", "type Result struct {\n flowBreaks []int // E2 overlay only; detached, never serialized.")
    text = replace(text, "return &Result{Frozen: frozen", "return &Result{flowBreaks:slices.Clone(breaks), Frozen: frozen")
    signature = "func finishLayout(ctx context.Context, owner *khipu.Khipukamayuq, encoded *khipu.Khipu, params Params, geometry khipu.Params, text styled.Text, config khipu.Config, runs []bidi.Segment, atomics map[contracts.AtomicID]AtomicBox) (*Result, error) {"
    helper = signature.replace("func finishLayout", "func flowFinishWithBreaks").replace("AtomicBox)", "AtomicBox, breaks []int)")
    text = replace(text, "positions, err := encoded.BreakpointPositions(breaks)", "return flowFinishWithBreaks(ctx,owner,encoded,params,geometry,text,config,runs,atomics,breaks)\n}\n\n" + helper + '\n fail := func(stage Stage, err error) (*Result, error) { return nil, &Error{stage,err} }\n positions, err := encoded.BreakpointPositions(breaks)')
    emit(path, text)

    for asset, target in [
        ("khipu.go.txt", KHIPU / "zz_flow_e2.go"),
        ("firstfit.go.txt", KHIPU / "linebreak/firstfit/zz_flow_e2.go"),
        ("paragraph.go.txt", ROOT / "glyphing/paragraph/zz_flow_e2.go"),
    ]:
        emit(target, (ASSETS / asset).read_text())
    for asset in sorted(ASSETS.glob("*_test.go.txt")):
        emit(ROOT / "internal/flowexperiment" / ("e2_" + asset.name.removesuffix(".txt")), asset.read_text())
    overlay = destination / "overlay.json"
    overlay.write_text(json.dumps({"Replace": mapping}, indent=2) + "\n")
    print(overlay)


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    generate(Path(sys.argv[1]).resolve())
