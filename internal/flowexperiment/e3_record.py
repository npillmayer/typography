#!/usr/bin/env python3
"""Record the host, exact commands, sources and font hashes before E3 samples.

Usage: python3 internal/flowexperiment/e3_record.py OVERLAY_JSON RESULTS_DIRECTORY
Run five sequential copies of the recorded command, redirecting each to sample-N.log.
"""
import datetime
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]


def command(args, cwd=ROOT):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    overlay, output = Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve()
    output.mkdir(parents=True, exist_ok=True)
    if (output/"environment.json").exists():
        sys.exit("choose a fresh evidence directory; do not overwrite previous measurements")
    modules = {}
    for name in ["typography", "khipu", "opentype", "uax", "cords", "schuko", "tree"]:
        root = ROOT.parent/name
        files = sorted(set(root.rglob("*.go")) | set(root.rglob("*.go.txt")))
        files += [f for f in [root/"go.mod", root/"go.sum"] if f.exists()]
        sources = {str(f.relative_to(root)): hashlib.sha256(f.read_bytes()).hexdigest() for f in files}
        modules[name] = {"head": command(["git","rev-parse","HEAD"],root),
                         "status": command(["git","status","--short"],root), "source_sha256": sources}
    fonts = {}
    for name in ["GentiumPlus-R.ttf", "NotoSansHebrew-Regular.ttf"]:
        path = ROOT.parent/"opentype/testdata/fonts"/name
        fonts[name] = hashlib.sha256(path.read_bytes()).hexdigest()
    replacements = json.loads(overlay.read_text())["Replace"]
    overlay_hashes = {target: hashlib.sha256(Path(source).read_bytes()).hexdigest() for target, source in replacements.items()}
    scripts = {f.name: hashlib.sha256(f.read_bytes()).hexdigest() for f in Path(__file__).parent.glob("e[23]_*.py")}
    data = {"recorded_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "os": platform.platform(), "go": command(["go","version"]),
            "go_env": json.loads(command(["go","env","-json","GOOS","GOARCH","GOFLAGS","CGO_ENABLED"])),
            "cpu": "See cpu: header in each raw benchmark log", "GOMAXPROCS": 1,
            "samples": 5, "benchtime": "50ms", "font_loading": "outside timing; stable warmed services",
            "command": ["go","test","-overlay",str(overlay),"./internal/flowexperiment","-run","^$","-bench","^BenchmarkE3","-benchmem","-benchtime=50ms","-count=1","-cpu=1","-timeout=10m"],
            "modules": modules, "fonts_sha256": fonts, "overlay_sha256": overlay_hashes, "scripts_sha256": scripts}
    (output/"environment.json").write_text(json.dumps(data,indent=2)+"\n")
    print(output/"environment.json")
