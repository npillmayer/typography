import copy
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("analyze", Path(__file__).with_name("e3_analyze.py"))
analyze = importlib.util.module_from_spec(spec)
spec.loader.exec_module(analyze)


class Measurements(unittest.TestCase):
    def complete(self):
        rows = [f"{name}\t2\t10 ns/op\t100 B/op\t3 allocs/op" for name in sorted(analyze.expected_names())]
        return "\n".join(rows+["PASS"])

    def test_missing_failed_and_duplicate_rows_rejected(self):
        text = self.complete()
        for broken in [text.replace("PASS", "FAIL"), "\n".join(text.splitlines()[1:]), text+"\n"+text.splitlines()[0]]:
            with self.assertRaises(ValueError):
                analyze.parse_sample(broken)

    def test_workload_weighting_and_dispersion(self):
        self.assertEqual(analyze.weighted([10,20], [110,120], .5), [60,70])
        self.assertAlmostEqual(analyze.weighted([10], [110], .9)[0], 20)
        dist = analyze.distribution([1,2,3,4,10])
        self.assertEqual((dist["min"],dist["median"],dist["max"]), (1,3,10))

    def test_five_process_samples_required(self):
        sample = analyze.parse_sample(self.complete())
        with self.assertRaises(ValueError):
            analyze.summarize([sample]*4)
        result = analyze.summarize([copy.deepcopy(sample) for _ in range(5)])
        self.assertEqual(result["case_count"], 121)
        for group in result["mixtures"].values():
            self.assertEqual(group["time_ratio"]["median"], 1)


if __name__ == "__main__":
    unittest.main()
