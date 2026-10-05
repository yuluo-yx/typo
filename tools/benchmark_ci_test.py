"""Regression tests for benchmark selection, sampling, and confirmation."""

import contextlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

import benchmark_ci as ci


def table(name="TypoCLI/fix-no-match-4", percent="13.01", p="0.000", n=10, metric="sec/op"):
    return (f"│ baseline.txt │ current.txt │\n│ {metric} │ {metric} vs base │\n"
            f"{name} 2.349m ± 1% 2.654m ± 2% +{percent}% (p={p} n={n})\n")


class ComparisonTests(unittest.TestCase):
    def test_original_alert_and_all_supported_metrics(self):
        report = "".join(table(metric=metric) for metric in ("sec/op", "B/op", "allocs/op"))
        self.assertEqual(len(ci.regressions(report)), 3)

    def test_threshold_significance_and_sample_boundaries(self):
        for percent, p, n, expected in (
            ("10.00", "0.049", 6, True), ("9.99", "0.000", 10, False),
            ("15", "0.050", 10, False), ("15", "1e-3", 10, True),
            ("15", "0.001", 5, False),
        ):
            with self.subTest(percent=percent, p=p, n=n):
                self.assertEqual(bool(ci.regressions(table(percent=percent, p=p, n=n))), expected)

    def test_non_regression_rows(self):
        report = table().replace("+13.01% (p=0.000 n=10)", "~ (p=0.500 n=10)")
        self.assertFalse(ci.regressions(report))
        self.assertFalse(ci.regressions(table().replace("+13.01%", "-13.01%")))

    def test_invalid_comparison_fails(self):
        for value in ("", "benchstat failed", "TypoCLI/no-match-4 +20% (p=0.001 n=10)"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                ci.regressions(value)

    def test_benchmark_pattern_escapes_names_and_preserves_hierarchy(self):
        pattern = ci.benchmark_pattern(["TypoCLI/a+b-4", "TypoCLI/fix-no-match-4", "Distance/short-4"])
        levels = pattern.split("/")
        self.assertTrue(re.fullmatch(levels[0], "BenchmarkTypoCLI"))
        self.assertTrue(re.fullmatch(levels[0], "BenchmarkDistance"))
        self.assertTrue(re.fullmatch(levels[1], "a+b"))
        self.assertFalse(re.fullmatch(levels[1], "aaab"))
        self.assertFalse(re.fullmatch(levels[1], "fix-no-match-extra"))

    def test_detection_requires_same_benchmark_and_metric_in_both_passes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            initial, confirmation, output, report = (root / name for name in ("initial", "confirmation", "output", "report"))
            initial.write_text(table() + table(metric="B/op"), encoding="utf-8")
            confirmation.write_text(table(metric="B/op") + table(name="TypoCLI/other-4"), encoding="utf-8")
            ci.detect(initial, output, report, confirmation)
            self.assertIn("regression=true", output.read_text())
            self.assertTrue(report.read_text().startswith("B/op:"))
            self.assertNotIn("sec/op:", report.read_text())
            self.assertIn("BenchmarkTypoCLI", output.read_text())

    def test_detection_drops_transient_alert(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            initial, confirmation, output, report = (root / name for name in ("initial", "confirmation", "output", "report"))
            initial.write_text(table(), encoding="utf-8")
            confirmation.write_text(table(percent="2.00"), encoding="utf-8")
            ci.detect(initial, output, report, confirmation)
            self.assertEqual(report.read_text(), "")
            self.assertEqual(output.read_text(), "regression=false\nbenchmark_pattern=\n")

    def test_initial_detection_preserves_candidates(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            initial, output, report = (root / name for name in ("initial", "output", "report"))
            initial.write_text(table(), encoding="utf-8")
            ci.detect(initial, output, report)
            self.assertIn("sec/op:", report.read_text())
            self.assertIn("regression=true", output.read_text())


class GitTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.chdir = contextlib.chdir(self.repo)
        self.chdir.__enter__()
        self.addCleanup(self.chdir.__exit__, None, None, None)
        ci.command(["git", "init", "-q"])
        self.commit_file("cmd/main.go", "package main\n")
        self.first = ci.command(["git", "rev-parse", "HEAD"]).strip()

    def commit_file(self, filename, content):
        path = self.repo / filename
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")
        ci.command(["git", "add", filename])
        ci.command(["git", "-c", "user.name=Benchmark Test", "-c", "user.email=benchmark@example.invalid",
                    "-c", "commit.gpgsign=false", "commit", "-qm", "test change"])

    def state(self, sha):
        path = self.root / "state.env"
        path.write_text(f"updated_at=test\nsha={sha}\n", encoding="utf-8")
        return path

    def test_first_commit_has_no_baseline(self):
        result = ci.select_baseline(self.root / "missing")
        self.assertEqual(result["should_compare"], "false")
        self.assertEqual(result["baseline_ref"], "")

    def test_issue_255_workflow_only_diff_skips(self):
        self.commit_file(".github/workflows/codeql.yml", "name: CodeQL\n")
        result = ci.select_baseline(self.state(self.first))
        self.assertEqual(result["should_compare"], "false")
        self.assertIn("unchanged", result["skip_reason"])

    def test_code_and_embedded_asset_changes_compare(self):
        for path in ("cmd/main.go", "internal/parser/data.json", "benchmarks/bench.sh", "tools/github-setup-deps/action.yml"):
            with self.subTest(path=path):
                self.commit_file(path, "changed\n")
                self.assertEqual(ci.select_baseline(self.state(self.first))["should_compare"], "true")

    def test_repeat_commit_skips_unless_forced(self):
        self.commit_file("README.md", "documentation\n")
        state = self.state(ci.command(["git", "rev-parse", "HEAD"]).strip())
        self.assertIn("already benchmarked", ci.select_baseline(state)["skip_reason"])
        self.assertEqual(ci.select_baseline(state, force=True)["should_compare"], "true")

    def test_invalid_cache_falls_back_to_parent(self):
        self.commit_file("cmd/main.go", "updated\n")
        result = ci.select_baseline(self.state("f" * 40))
        self.assertEqual(result["baseline_ref"], self.first)
        self.assertEqual(result["should_compare"], "true")

    def test_empty_state_falls_back_to_parent(self):
        self.commit_file("cmd/main.go", "updated\n")
        path = self.root / "state.env"
        path.write_text("updated_at=test\n", encoding="utf-8")
        self.assertEqual(ci.select_baseline(path)["baseline_ref"], self.first)

    def install_fake_go(self):
        binary_dir = self.root / "bin"
        binary_dir.mkdir()
        binary = binary_dir / "go"
        binary.write_text(f"#!{sys.executable}\n" + '''import json, os, sys
from pathlib import Path
with Path(os.environ["BENCHMARK_TEST_LOG"]).open("a") as log:
    log.write(json.dumps({"args": sys.argv[1:], "cwd": os.getcwd(),
                          "GOMAXPROCS": os.environ.get("GOMAXPROCS"),
                          "GOFLAGS": os.environ.get("GOFLAGS")}) + "\\n")
if sys.argv[1] == "version":
    print("go version go1.26.6 test/amd64")
elif sys.argv[1] == "test":
    if "-benchtime=1x" not in sys.argv and os.environ.get("BENCHMARK_TEST_FAIL"):
        print("simulated benchmark failure", file=sys.stderr)
        sys.exit(1)
    if not os.environ.get("BENCHMARK_TEST_EMPTY"):
        print("BenchmarkTypoCLI/fix-no-match-4 1 100 ns/op 10 B/op 1 allocs/op")
    print("PASS")
''', encoding="utf-8")
        binary.chmod(0o755)
        log = self.root / "go.jsonl"
        return mock.patch.dict(os.environ, {"PATH": str(binary_dir) + os.pathsep + os.environ["PATH"],
                                           "BENCHMARK_TEST_LOG": str(log)}), log

    def test_interleaved_sampling_warmup_and_reverse_order(self):
        patch, log = self.install_fake_go()
        for reverse in (False, True):
            with self.subTest(reverse=reverse), patch, contextlib.redirect_stdout(io.StringIO()):
                output = self.root / str(reverse)
                ci.sample(self.first, "HEAD", output, samples=6, reverse=reverse)
                metadata = json.loads((output / "environment.json").read_text())
                labels = [run["label"] for run in metadata["runs"]]
                expected = ["baseline", "current", "current", "baseline"]
                if reverse:
                    expected = ["current", "baseline", "baseline", "current"]
                self.assertEqual(labels, expected * 3)
                for label in ("baseline", "current"):
                    self.assertEqual((output / f"{label}.txt").read_text().count("BenchmarkTypoCLI"), 6)
                self.assertEqual(metadata["GOMAXPROCS"], "4")
                self.assertEqual(len(ci.command(["git", "worktree", "list", "--porcelain"]).split("worktree ")), 2)
        commands = [json.loads(line) for line in log.read_text().splitlines()]
        self.assertEqual(sum("-benchtime=1x" in run["args"] for run in commands), 4)
        self.assertTrue(all(run["GOFLAGS"] == "-buildvcs=false" for run in commands))

    def test_benchmark_failure_propagates_and_cleans_worktrees(self):
        patch, _ = self.install_fake_go()
        with patch, mock.patch.dict(os.environ, {"BENCHMARK_TEST_FAIL": "1"}), self.assertRaisesRegex(RuntimeError, "simulated benchmark failure"):
            ci.sample(self.first, "HEAD", self.root / "failed", samples=6)
        self.assertTrue((self.root / "failed/environment.json").is_file())
        self.assertEqual(len(ci.command(["git", "worktree", "list", "--porcelain"]).split("worktree ")), 2)

    def test_empty_benchmark_output_is_an_error(self):
        patch, _ = self.install_fake_go()
        with patch, mock.patch.dict(os.environ, {"BENCHMARK_TEST_EMPTY": "1"}), self.assertRaisesRegex(RuntimeError, "No benchmark samples"):
            ci.sample(self.first, "HEAD", self.root / "empty", samples=6)

    def test_worktree_setup_failure_cleans_first_worktree(self):
        patch, _ = self.install_fake_go()
        with patch, self.assertRaisesRegex(RuntimeError, "Command failed"):
            ci.sample(self.first, "missing-ref", self.root / "failed", samples=6)
        self.assertEqual(len(ci.command(["git", "worktree", "list", "--porcelain"]).split("worktree ")), 2)


class CommandTests(unittest.TestCase):
    def test_invalid_sample_count_is_rejected(self):
        for count in (0, 5, 7):
            with self.subTest(count=count), self.assertRaises(ValueError):
                ci.sample("HEAD", "HEAD", "unused", samples=count)

    def test_main_dispatch(self):
        invocations = (
            (["select", "--output", "output"], "select_baseline"),
            (["sample", "--baseline", "HEAD", "--output-dir", "results"], "sample"),
            (["detect", "--initial", "comparison", "--output", "output", "--report", "report"], "detect"),
        )
        for arguments, function in invocations:
            with self.subTest(function=function), mock.patch.object(sys, "argv", ["benchmark_ci.py", *arguments]), \
                    mock.patch.object(ci, function, return_value={}) as called, mock.patch.object(ci, "write_outputs"):
                ci.main()
                called.assert_called_once()


class WorkflowTests(unittest.TestCase):
    def workflow_script(self, step):
        workflow = (Path(__file__).resolve().parents[1] / ".github/workflows/benchmark.yml").read_text()
        block = workflow.split(f"      - name: {step}\n", 1)[1].split("      - name:", 1)[0]
        lines = block.split("        run: |\n", 1)[1].splitlines()
        return "\n".join(line[10:] for line in lines if line.startswith("          "))

    def test_state_retains_baseline_on_confirmed_regression_even_without_cache(self):
        with tempfile.TemporaryDirectory() as directory:
            for confirmed, expected in (("true", "baseline"), ("false", "current"), ("", "current")):
                with self.subTest(confirmed=confirmed):
                    env = dict(os.environ, CURRENT_SHA="current", BASELINE_SHA="baseline", CONFIRMED_REGRESSION=confirmed)
                    subprocess.run(["bash", "-e", "-o", "pipefail", "-c", self.workflow_script("Update benchmark state")],
                                   cwd=directory, env=env, check=True)
                    self.assertIn(f"sha={expected}\n", (Path(directory) / "benchmark-state.env").read_text())

    def test_benchstat_failure_stops_comparison(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "benchmark-results/initial").mkdir(parents=True)
            binary = root / "benchstat"
            binary.write_text("#!/bin/sh\nexit 23\n", encoding="utf-8")
            binary.chmod(0o755)
            env = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"])
            result = subprocess.run(["bash", "-e", "-o", "pipefail", "-c", self.workflow_script("Compare with baseline")],
                                    cwd=root, env=env, capture_output=True)
            self.assertEqual(result.returncode, 23)


if __name__ == "__main__":
    unittest.main()
