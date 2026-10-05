"""Select, sample, and confirm benchmark comparisons with standard-library tools."""

import argparse
import json
import os
from pathlib import Path
import platform
import re
import shlex
import subprocess
import tempfile
import time


BENCHMARK_INPUTS = (
    "*.go", "cmd/", "internal/", "go.mod", "go.sum", "benchmarks/",
    "tools/github-setup-deps/", "tools/benchmark_ci.py",
    ".github/workflows/benchmark.yml", ".mise.toml", "mise.toml",
)
METRIC_HEADER = re.compile(r"│\s*(sec/op|B/op|allocs/op)\s*│")
REGRESSION_ROW = re.compile(
    r"^\s*(\S+)\s+.*?\+(\d+(?:\.\d+)?)%.*?\(p=([\d.eE+-]+)\s+n=(\d+)\)"
)


def command(args, cwd=None, env=None):
    result = subprocess.run(args, cwd=cwd, env=env, text=True,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f"Command failed: {shlex.join(args)}\n{result.stdout}")
    return result.stdout


def write_outputs(path, values):
    with Path(path).open("a", encoding="utf-8") as output:
        for key, value in values.items():
            output.write(f"{key}={value}\n")


def select_baseline(state, force=False):
    current = command(["git", "rev-parse", "HEAD"]).strip()
    previous = ""
    if Path(state).is_file():
        for line in Path(state).read_text(encoding="utf-8").splitlines():
            if line.startswith("sha="):
                previous = line[4:]
                break
    reference = "HEAD^" if force or previous in ("", current) else previous
    try:
        baseline = command(["git", "rev-parse", "--verify", f"{reference}^{{commit}}"]).strip()
    except RuntimeError:
        try:
            baseline = command(["git", "rev-parse", "--verify", "HEAD^"]).strip()
        except RuntimeError:
            baseline = ""

    reason = ""
    if not baseline or baseline == current:
        reason = "no baseline commit is available"
    elif not force and previous == current:
        reason = "current commit was already benchmarked"
    elif not force and not command(
        ["git", "diff", "--name-only", baseline, current, "--", *BENCHMARK_INPUTS]
    ).strip():
        reason = "application, dependencies, benchmark, and toolchain inputs are unchanged"
    return {"current_sha": current, "baseline_ref": baseline,
            "should_compare": str(not reason).lower(), "skip_reason": reason}


def sample(baseline_ref, current_ref, output_dir, pattern=".", samples=10,
           benchtime="1s", reverse=False):
    if samples < 6 or samples % 2:
        raise ValueError("Sample count must be even and at least 6")
    output_dir = Path(output_dir).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env.update(GOMAXPROCS="4", GOFLAGS="-buildvcs=false")
    metadata = {"platform": platform.platform(), "go_version": command(["go", "version"], env=env).strip(),
                "baseline_ref": baseline_ref, "current_ref": current_ref,
                "GOMAXPROCS": env["GOMAXPROCS"], "GOFLAGS": env["GOFLAGS"],
                "pattern": pattern, "samples": samples, "benchtime": benchtime, "runs": []}
    command(["git", "rev-parse", "--show-toplevel"])
    with tempfile.TemporaryDirectory(prefix="typo-benchmark-") as temporary:
        # Equal-length clean worktree paths avoid parent-process allocation differences.
        directories = {"baseline": Path(temporary) / "baseline", "current": Path(temporary) / "current_"}
        added = []
        try:
            for label, reference in (("baseline", baseline_ref), ("current", current_ref)):
                command(["git", "worktree", "add", "--detach", str(directories[label]), reference])
                added.append(directories[label])
                command(["go", "mod", "download"], cwd=directories[label], env=env)
                # Warm both builds and fixtures before collecting samples.
                command(["go", "test", f"-bench={pattern}", "-benchmem", "-count=1", "-run=^$",
                         "-benchtime=1x", "-timeout=30m", "./benchmarks/"], cwd=directories[label], env=env)
            for label in directories:
                (output_dir / f"{label}.txt").write_text("", encoding="utf-8")
            for index in range(samples):
                order = ("baseline", "current") if (index + int(reverse)) % 2 == 0 else ("current", "baseline")
                for label in order:
                    started = time.time()
                    output = command(
                        ["go", "test", f"-bench={pattern}", "-benchmem", "-count=1", "-run=^$",
                         f"-benchtime={benchtime}", "-timeout=30m", "./benchmarks/"],
                        cwd=directories[label], env=env,
                    )
                    if not re.search(r"^Benchmark\S+\s+\d+\s+", output, re.MULTILINE):
                        raise RuntimeError("No benchmark samples were produced")
                    with (output_dir / f"{label}.txt").open("a", encoding="utf-8") as results:
                        results.write(output)
                    metadata["runs"].append({"round": index + 1, "label": label,
                                             "started_at": started, "elapsed_seconds": time.time() - started})
                    print(f"Round {index + 1}/{samples}: {label}\n{output}", flush=True)
        finally:
            (output_dir / "environment.json").write_text(json.dumps(metadata, indent=2) + "\n", encoding="utf-8")
            for directory in reversed(added):
                command(["git", "worktree", "remove", "--force", str(directory)])


def regressions(comparison, threshold=10):
    metric = None
    found_header = False
    result = {}
    for line in comparison.splitlines():
        header = METRIC_HEADER.search(line)
        if header:
            metric = header[1]
            found_header = True
        match = REGRESSION_ROW.match(line)
        if metric and match and float(match[2]) >= threshold and float(match[3]) < 0.05 and int(match[4]) >= 6:
            result[(match[1], metric)] = line.strip()
    if not found_header:
        raise ValueError("No supported benchstat metric table was found")
    return result


def benchmark_pattern(names):
    # Go splits -bench expressions at slashes; group alternatives at each level.
    parts = [re.sub(r"-\d+$", "", name).split("/") for name in sorted(set(names))]
    levels = []
    for index in range(max(len(part) for part in parts)):
        values = sorted({("Benchmark" if index == 0 else "") + part[index]
                         for part in parts if index < len(part)})
        levels.append("^(" + "|".join(re.escape(value) for value in values) + ")$")
    return "/".join(levels)


def detect(initial, output, report, confirmation=None):
    candidates = regressions(Path(initial).read_text(encoding="utf-8"))
    if confirmation:
        repeated = regressions(Path(confirmation).read_text(encoding="utf-8"))
        candidates = {key: line for key, line in candidates.items() if key in repeated}
    lines = [f"{metric}: {line}" for (_, metric), line in candidates.items()]
    Path(report).write_text("\n".join(lines) + ("\n" if lines else ""), encoding="utf-8")
    write_outputs(output, {"regression": str(bool(candidates)).lower(),
                           "benchmark_pattern": benchmark_pattern(key[0] for key in candidates) if candidates else ""})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="action", required=True)
    select = subparsers.add_parser("select")
    select.add_argument("--state", default="benchmark-state.env")
    select.add_argument("--force", action="store_true")
    select.add_argument("--output", required=True)
    run = subparsers.add_parser("sample")
    run.add_argument("--baseline", required=True)
    run.add_argument("--current", default="HEAD")
    run.add_argument("--output-dir", required=True)
    run.add_argument("--pattern", default=".")
    run.add_argument("--samples", type=int, default=10)
    run.add_argument("--benchtime", default="1s")
    run.add_argument("--reverse", action="store_true")
    compare = subparsers.add_parser("detect")
    compare.add_argument("--initial", required=True)
    compare.add_argument("--confirmation")
    compare.add_argument("--output", required=True)
    compare.add_argument("--report", required=True)
    args = parser.parse_args()
    if args.action == "select":
        write_outputs(args.output, select_baseline(args.state, args.force))
    elif args.action == "sample":
        sample(args.baseline, args.current, args.output_dir, args.pattern, args.samples, args.benchtime, args.reverse)
    else:
        detect(args.initial, args.output, args.report, args.confirmation)


if __name__ == "__main__":
    main()
