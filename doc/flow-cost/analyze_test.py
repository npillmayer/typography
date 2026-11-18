import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("analyze", Path(__file__).with_name("analyze.py"))
analyze = importlib.util.module_from_spec(spec)
spec.loader.exec_module(analyze)
EVIDENCE = Path(__file__).parent / "results/2026-11-18"


class Measurements(unittest.TestCase):
    def complete(self):
        # Real recorded rows supply independent expected metric inventories.
        return (EVIDENCE / "sample-1.log").read_text().replace(
            "accepted-lines/op", "original-cut-lines/op").replace(
            "accepted-percent", "original-cut-percent")

    def rows(self, sample):
        return "\n".join(f"{name} {row['iterations']} " + " ".join(
            f"{v} {k}" for k, v in row["metrics"].items())
            for name, row in sample.items()) + "\nPASS\n"

    def test_missing_failed_and_duplicate_rows_rejected(self):
        text = self.complete()
        row = next(line for line in text.splitlines() if line.startswith("BenchmarkP6"))
        for broken in [text.replace("PASS", "FAIL"), text.replace(row, ""), text+"\n"+row]:
            with self.assertRaises(ValueError):
                analyze.parse_sample(broken)

    def test_every_required_metric_in_each_family(self):
        original = analyze.parse_sample(self.complete())
        for name in ["BenchmarkP6Prepare/synthetic/N100", "BenchmarkP6Prepare/gentium/N100",
                     "BenchmarkP6Initial/synthetic/N100", "BenchmarkP6Initial/gentium/N100",
                     "BenchmarkP6Frames/synthetic/N100/P50/equal", "BenchmarkP6Frames/hebrew/N100/P50/continue",
                     "BenchmarkP6FixedSuffix/N1000/checkpoint", "BenchmarkP6FixedSuffix/N1000/phases"]:
            for metric in original[name]["metrics"]:
                with self.subTest(name=name, metric=metric):
                    broken = copy.deepcopy(original)
                    del broken[name]["metrics"][metric]
                    with self.assertRaises(ValueError):
                        analyze.parse_sample(self.rows(broken))
                    # Consistent omissions across all five processes must also fail.
                    with self.assertRaises(ValueError):
                        analyze.summarize([broken]*5)

    def test_zero_call_gates(self):
        original = analyze.parse_sample(self.complete())
        streams = ["factories/op", "shapes/op", "acceptance/op"]
        extra = ["resolutions/op", "metrics/op", "scales/op", "conversions/op"]
        for name, row in original.items():
            if name.startswith("BenchmarkP6Prepare/"):
                continue
            gates = list(streams)
            if name.endswith("/checkpoint") or ("/synthetic/" in name and name.endswith("/equal")):
                gates += extra
            if name.endswith("/phases"):
                gates += ["prefix-width-queries/op"]
            for metric in gates:
                with self.subTest(name=name, metric=metric):
                    broken = dict(row["metrics"], **{metric: 1})
                    with self.assertRaisesRegex(ValueError, "zero-call gate"):
                        analyze.validate_metrics(name, broken)
        # Preparation service work and changed-width assembly work are legitimate.
        self.assertGreater(original["BenchmarkP6Prepare/synthetic/N100"]["metrics"]["shapes/op"], 0)
        self.assertGreater(original["BenchmarkP6Frames/synthetic/N100/P50/continue"]["metrics"]["metrics/op"], 0)

    def test_stripped_counters_and_invalid_values(self):
        sample = analyze.parse_sample(self.complete())
        for row in sample.values():
            row["metrics"] = {k:v for k,v in row["metrics"].items() if k in ("ns/op", "B/op", "allocs/op")}
        with self.assertRaises(ValueError):
            analyze.summarize([sample]*5)
        name = "BenchmarkP6Prepare/gentium/N100"
        for value in [0, -1, float("nan"), float("inf")]:
            metrics = analyze.parse_sample(self.complete())[name]["metrics"]
            metrics["ns/op"] = value
            with self.assertRaises(ValueError):
                analyze.validate_metrics(name, metrics)

    def test_cut_labels_and_historical_summary(self):
        samples = [analyze.parse_sample((EVIDENCE/f"sample-{i}.log").read_text()) for i in range(1,6)]
        self.assertEqual(analyze.summarize(samples), json.loads((EVIDENCE/"summary.json").read_text()))
        current = analyze.parse_sample(self.complete())
        name = "BenchmarkP6Frames/synthetic/N100/P50/relayout-narrow"
        # This is 5/10 ORIGINAL lines, not 5/13 relaid-out lines.
        self.assertEqual(current[name]["metrics"]["original-cut-percent"], 50)
        for mutate in [lambda m: m.update({"accepted-percent": 50}),
                       lambda m: m.update({"accepted-percent": m.pop("original-cut-percent")})]:
            sample = copy.deepcopy(current)
            mutate(sample[name]["metrics"])
            with self.assertRaises(ValueError):
                analyze.parse_sample(self.rows(sample))
        with self.assertRaises(ValueError):
            analyze.summarize(samples[:4]+[current])

    def test_workload_weighting_and_dispersion(self):
        self.assertEqual(analyze.weighted([10,20], [110,120], .5), [60,70])
        self.assertAlmostEqual(analyze.weighted([10], [110], .9)[0], 20)
        dist = analyze.distribution([1,2,3,4,10])
        self.assertEqual((dist["min"],dist["median"],dist["max"]), (1,3,10))
        sample = analyze.parse_sample(self.complete())
        with self.assertRaises(ValueError):
            analyze.summarize([sample]*4)
        self.assertEqual(analyze.summarize([sample]*5)["case_count"], 104)

    def test_cli_never_overwrites_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            target = Path(tmp)/"summary.json"
            target.write_text("previous evidence")
            result = subprocess.run([sys.executable, str(Path(analyze.__file__)), str(EVIDENCE), str(target)], capture_output=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(target.read_text(), "previous evidence")


if __name__ == "__main__":
    unittest.main()
