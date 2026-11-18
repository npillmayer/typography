#!/usr/bin/env python3
"""Run five serial fresh Go processes, recording immutable production cost evidence.

Usage from typography: python3 doc/flow-cost/record.py NEW_RESULTS_DIRECTORY
No overlays, build tags, discovery or network fonts. Never overwrite prior evidence.
Run correctness checks separately, not concurrently with sampling.
"""
import datetime
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
COMMAND = ["go", "test", "./glyphing/paragraph", "-run", "^$", "-bench", "^BenchmarkP6", "-benchmem", "-benchtime=50ms", "-count=1", "-cpu=1", "-timeout=10m"]


def command(args, cwd=ROOT):
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    output = Path(sys.argv[1]).resolve()
    output.mkdir(parents=True, exist_ok=False)
    modules = {}
    for name in ["typography", "khipu", "tytui", "css-box-layout", "opentype", "uax", "cords", "schuko", "tree"]:
        root = ROOT.parent / name
        files = sorted(root.rglob("*.go")) + [root/"go.mod", root/"go.sum"]
        modules[name] = {"head": command(["git", "rev-parse", "HEAD"], root),
                         "status": command(["git", "status", "--short"], root),
                         "source_sha256": {str(f.relative_to(root)): hashlib.sha256(f.read_bytes()).hexdigest() for f in files if f.exists()}}
    data = {"recorded_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "os": platform.platform(), "go": command(["go", "version"]),
            "go_env": json.loads(command(["go", "env", "-json", "GOOS", "GOARCH", "GOFLAGS", "CGO_ENABLED"])),
            "cpu": "See cpu header in raw logs", "GOMAXPROCS": 1, "command": COMMAND,
            "samples": 5, "modules": modules,
            "scripts_sha256": {f.name: hashlib.sha256(f.read_bytes()).hexdigest() for f in Path(__file__).parent.glob("*.py")},
            "fonts_sha256": {name: hashlib.sha256((ROOT.parent/"opentype/testdata/fonts"/name).read_bytes()).hexdigest() for name in ["GentiumPlus-R.ttf", "NotoSansHebrew-Regular.ttf"]}}
    (output/"environment.json").write_text(json.dumps(data, indent=2)+"\n")
    for n in range(1, 6):
        print(f"Starting process {n}/5", flush=True)
        with (output/f"sample-{n}.log").open("x") as log:
            subprocess.run(COMMAND, cwd=ROOT, stdout=log, stderr=subprocess.STDOUT, check=True)
        print(f"Completed process {n}/5", flush=True)
    subprocess.run([sys.executable, str(Path(__file__).with_name("analyze.py")), str(output)], check=True)


if __name__ == "__main__":
    main()
